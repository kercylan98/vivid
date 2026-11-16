package actor

import (
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

type startupEventStream struct {
	vivid.EventStream
	observe func(vivid.Message)
}

func (s *startupEventStream) Publish(ctx vivid.EventStreamContext, event vivid.Message) {
	s.EventStream.Publish(ctx, event)
	s.observe(event)
}

func TestSystem_RemotingStartRollbackWithInbound(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	system := NewSystem(
		vivid.WithActorSystemRemoting(address),
		vivid.WithActorSystemStopTimeout(time.Second),
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterSeeds(address)),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
	)
	readerSpawned := make(chan struct{})
	var readerOnce sync.Once
	var inbound net.Conn
	defer func() {
		if inbound != nil {
			_ = inbound.Close()
		}
	}()
	// 在启动链继续执行 Gossip 前完成一次握手，固定“已有 Reader 等待启动信号”的窗口。
	system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
		spawned, ok := event.(ves.ActorSpawnedEvent)
		if !ok {
			return
		}
		if strings.Contains(spawned.ActorRef.GetPath(), "/reader-") {
			readerOnce.Do(func() { close(readerSpawned) })
			return
		}
		if spawned.ActorRef.GetPath() != "/@remoting" {
			return
		}
		conn, err := net.DialTimeout("tcp", address, time.Second)
		require.NoError(t, err)
		inbound = conn
		require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
		require.NoError(t, (&remoting.Handshake{AdvertiseAddr: "127.0.0.1:1"}).Send(conn))
		require.NoError(t, new(remoting.Handshake).Wait(conn))
		select {
		case <-readerSpawned:
		case <-time.After(time.Second):
			t.Fatal("inbound reader not spawned")
		}
	}}
	err = system.Start()
	require.ErrorIs(t, err, vivid.ErrorActorSystemStartFailed)
	require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
	require.NotErrorIs(t, err, vivid.ErrorActorSystemStopFailed, "回滚不得与 Reader 的启动等待互相阻塞")
	select {
	case <-system.guardClosedSignal:
	default:
		t.Fatal("启动失败返回前尚未完成清理")
	}
	require.Error(t, system.options.Context.Err())
	listener, err = net.Listen("tcp", address)
	require.NoError(t, err)
	require.NoError(t, listener.Close())
}

func TestSystem_StopDuringStartup(t *testing.T) {
	for _, component := range []string{"/@remoting", "/@gossip"} {
		t.Run(component, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			address := listener.Addr().String()
			require.NoError(t, listener.Close())
			system := NewSystem(
				vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
				vivid.WithActorSystemRemoting(address),
				vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")),
			)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			t.Cleanup(func() {
				once.Do(func() { close(release) })
				_ = system.Stop(time.Second)
			})
			system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
				spawned, ok := event.(ves.ActorSpawnedEvent)
				if ok && spawned.ActorRef.GetPath() == component {
					close(entered)
					<-release
				}
			}}
			started := make(chan error, 1)
			go func() { started <- system.Start() }()
			receiveRetryResult(t, entered)
			require.ErrorIs(t, system.Start(), vivid.ErrorActorSystemAlreadyStarted)
			stopped := make(chan error, 1)
			go func() { stopped <- system.Stop(time.Second) }()
			require.Eventually(t, func() bool {
				system.statusLock.Lock()
				defer system.statusLock.Unlock()
				return system.status == stopping
			}, time.Second, time.Millisecond)
			created := make(chan error, 1)
			go func() {
				_, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
				created <- err
			}()
			once.Do(func() { close(release) })
			require.ErrorIs(t, receiveRetryResult(t, started), vivid.ErrorActorSystemStopped, "Stop 已在初始化期间获胜时，Start 不应报告成功")
			require.NoError(t, receiveRetryResult(t, stopped), "Stop 应等待启动链结束后清理全部已创建组件")
			require.ErrorIs(t, receiveRetryResult(t, created), vivid.ErrorActorSystemStopped)
			receiveRetryResult(t, system.guardClosedSignal)
			require.Error(t, system.options.Context.Err())
			listener, err = net.Listen("tcp", address)
			require.NoError(t, err, "Stop 返回时启动期间创建的监听必须释放")
			require.NoError(t, listener.Close())
			require.ErrorIs(t, system.Start(), vivid.ErrorActorSystemAlreadyStopped)
		})
	}
}

func TestSystem_ActorOfDuringCodecRegistration(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseRegistration := func() { once.Do(func() { close(release) }) }
	system := NewSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		vivid.WithActorSystemMessageRegister(vivid.MessageRegisterFN(func(vivid.MessageRegistry) {
			close(entered)
			<-release
		})),
	)
	_, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
	require.ErrorIs(t, err, vivid.ErrorActorSystemNotStarted)

	t.Cleanup(func() {
		releaseRegistration()
		_ = system.Stop(time.Second)
	})
	started := make(chan error, 1)
	go func() { started <- system.Start() }()
	receiveRetryResult(t, entered)

	require.NotPanics(t, func() {
		_, err = system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
	})
	require.ErrorIs(t, err, vivid.ErrorActorSystemNotStarted)
	releaseRegistration()
	require.NoError(t, receiveRetryResult(t, started))
}
