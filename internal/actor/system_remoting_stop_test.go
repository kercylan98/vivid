package actor_test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func stopTestSystem(t *testing.T) *actor.System {
	t.Helper()
	system := actor.NewSystem(
		vivid.WithActorSystemRemoting(clusterTestAddress(t)),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
	)
	require.NoError(t, system.Start())
	t.Cleanup(func() {
		err := system.Stop(3 * time.Second)
		require.True(t, err == nil || errors.Is(err, vivid.ErrorActorSystemAlreadyStopped), "%v", err)
	})
	return system
}

func TestSystem_RemotingSilentInbound(t *testing.T) {
	system := stopTestSystem(t)
	conn, err := net.DialTimeout("tcp", system.Ref().GetAddress(), time.Second)
	require.NoError(t, err)
	defer conn.Close()
	// 发送不完整握手，确保 Acceptor 已进入读取阶段。
	_, err = conn.Write([]byte{0})
	require.NoError(t, err)
	require.NoError(t, system.Stop(time.Second), "静默入站握手不能阻塞系统关闭")
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	_, err = conn.Read(make([]byte, 1))
	require.Error(t, err)
	if timeout, ok := err.(net.Error); ok {
		require.False(t, timeout.Timeout(), "停止时必须释放待握手连接")
	}
}

func TestSystem_RemotingSilentPeerDoesNotBlockAccept(t *testing.T) {
	system := stopTestSystem(t)
	silent, err := net.DialTimeout("tcp", system.Ref().GetAddress(), time.Second)
	require.NoError(t, err)
	defer silent.Close()
	_, err = silent.Write([]byte{0})
	require.NoError(t, err)
	peer := stopTestSystem(t)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(vivid.ActorRef); ok {
			ctx.Reply(message)
		}
	}))
	require.NoError(t, err)
	_, err = peer.Ask(ref, peer.Ref(), time.Second).Result()
	require.NoError(t, err, "一个静默握手不得阻塞其他节点建立连接")
}

func TestSystem_RemotingStopOutboundHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	system := stopTestSystem(t)
	ref, err := system.CreateRef(listener.Addr().String(), "/silent")
	require.NoError(t, err)
	system.Tell(ref, system.Ref())
	var conn net.Conn
	select {
	case conn = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("outbound connection not accepted")
	}
	defer conn.Close()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	frame, err := remoting.ReadFrame(conn, nil, remoting.FrameReadLimits{MaxDataLen: 1024})
	require.NoError(t, err)
	require.Equal(t, remoting.FrameCtrlHandshake, frame.Type)
	require.NoError(t, system.Stop(time.Second))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	_, err = conn.Read(make([]byte, 1))
	require.Error(t, err)
	if timeout, ok := err.(net.Error); ok {
		require.False(t, timeout.Timeout(), "停止后不能遗留出站握手协程及连接")
	}
}

func TestSystem_RemotingHandshakeCapacityRecovery(t *testing.T) {
	system := stopTestSystem(t)
	var silent []net.Conn
	defer func() {
		for _, conn := range silent {
			_ = conn.Close()
		}
	}()
	// 填满当前 64 个入站握手请求名额，验证暂停接入后仍能恢复。
	for range 64 {
		conn, err := net.DialTimeout("tcp", system.Ref().GetAddress(), time.Second)
		require.NoError(t, err)
		silent = append(silent, conn)
		_, err = conn.Write([]byte{0})
		require.NoError(t, err)
	}
	peer := stopTestSystem(t)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(vivid.ActorRef); ok {
			ctx.Reply(message)
		}
	}))
	require.NoError(t, err)
	completed := make(chan error, 1)
	go func() {
		_, err := peer.Ask(ref, peer.Ref(), 2*time.Second).Result()
		completed <- err
	}()
	select {
	case err := <-completed:
		t.Fatalf("握手名额已满时仍接入了新连接: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	require.NoError(t, silent[0].Close())
	require.NoError(t, receiveTellResult(t, completed), "释放握手名额后应继续接入，无需等待其他静默连接超时")
}
