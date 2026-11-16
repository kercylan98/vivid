package actor

import (
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/testkit"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

type admissionEnvelopHandler struct {
	remoting.NetworkEnvelopHandler
	failed chan vivid.Envelop
}

func (h *admissionEnvelopHandler) HandleFailedRemotingEnvelop(envelop vivid.Envelop) {
	h.NetworkEnvelopHandler.HandleFailedRemotingEnvelop(envelop)
	h.failed <- envelop
}

func admissionSystemAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func newAdmissionSystem(t *testing.T, config *tls.Config, perAddress, total int) (*System, <-chan vivid.Envelop, <-chan error) {
	t.Helper()
	system := NewSystem(
		vivid.WithActorSystemRemoting(admissionSystemAddress(t)),
		vivid.WithActorSystemRemotingOption(
			vivid.WithActorSystemRemotingMaxPendingEnvelops(perAddress),
			vivid.WithActorSystemRemotingMaxTotalPendingEnvelops(total),
			vivid.WithActorSystemRemotingTLSConfig(config),
			vivid.WithActorSystemRemotingReconnectLimit(0),
		),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
	)
	failed, rejected := make(chan vivid.Envelop, 512), make(chan error, 512)
	system.remotingEnvelopHandler = &admissionEnvelopHandler{NetworkEnvelopHandler: system.remotingEnvelopHandler, failed: failed}
	system.eventStream = &retryEventStream{EventStream: system.eventStream, observe: func(_ vivid.EventStreamContext, event vivid.Message) {
		if message, ok := event.(ves.RemotingEnvelopSendFailedEvent); ok {
			rejected <- message.Error
		}
	}}
	require.NoError(t, system.Start())
	t.Cleanup(func() {
		err := system.Stop(time.Second)
		require.True(t, err == nil || errors.Is(err, vivid.ErrorActorSystemAlreadyStopped), "%v", err)
	})
	return system, failed, rejected
}

func TestSystem_RemotingAdmissionBeforeMailbox(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, secure := range []bool{false, true} {
		for _, stop := range []bool{false, true} {
			t.Run(map[bool]string{false: "tcp", true: "mtls"}[secure]+map[bool]string{false: "/resume", true: "/stop"}[stop], func(t *testing.T) {
				var localTLS, peerTLS *tls.Config
				if secure {
					localTLS, peerTLS = ca.Config(t, true), ca.Config(t, true)
				}
				system, failed, rejected := newAdmissionSystem(t, localTLS, 2, 3)
				value, exists := system.actorContexts.Load(system.remotingRef.GetPath())
				require.True(t, exists)
				remoteContext := value.(*Context)
				remoteContext.mailbox.Pause()
				t.Cleanup(remoteContext.mailbox.Resume)
				var targets []vivid.ActorRef
				accepted := make(chan net.Conn, 4)
				for range 2 {
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					require.NoError(t, err)
					t.Cleanup(func() { _ = listener.Close() })
					go func() {
						conn, err := listener.Accept()
						if err == nil {
							accepted <- conn // 保持静默，让消息一直占用写入前的名额。
						}
					}()
					ref, err := system.CreateRef(listener.Addr().String(), "/blocked")
					require.NoError(t, err)
					targets = append(targets, ref)
				}
				otherRef, err := system.CreateRef(targets[0].GetAddress(), "/another")
				require.NoError(t, err)
				original := mailbox.NewEnvelop(true, system.Ref(), targets[0], system.Ref())
				system.ResolveMailbox(targets[0]).Enqueue(original)
				system.Tell(otherRef, system.Ref())
				system.Tell(targets[1], system.Ref())
				system.Tell(targets[0], system.Ref())
				require.ErrorIs(t, receiveRetryResult(t, rejected), vivid.ErrorRemotingPendingEnvelopsFull)
				require.True(t, targets[0].Equals(receiveRetryResult(t, failed).Receiver()))
				thirdRef, err := system.CreateRef("127.0.0.1:1", "/overflow")
				require.NoError(t, err)
				var group sync.WaitGroup
				for range 64 {
					group.Go(func() { system.Tell(thirdRef, system.Ref()) })
				}
				group.Wait()
				for range 64 {
					require.ErrorIs(t, receiveRetryResult(t, rejected), vivid.ErrorRemotingTotalPendingEnvelopsFull)
					require.True(t, thirdRef.Equals(receiveRetryResult(t, failed).Receiver()))
				}
				require.Empty(t, accepted, "暂停消费时拒绝必须已生效，不能等端点处理后才检查")
				if stop {
					// 使用系统终止消息固定组件先停止、普通信封仍未处理的窗口。
					system.Kill(system.remotingRef, false, "admission stop test")
				} else {
					remoteContext.mailbox.Resume()
					for range 2 {
						conn := receiveRetryResult(t, accepted)
						require.NoError(t, conn.Close())
					}
				}
				var sawOriginal bool
				for range 3 {
					envelop := receiveRetryResult(t, failed)
					require.True(t, system.Ref().Equals(envelop.Message().(vivid.ActorRef)))
					if envelop == original {
						sawOriginal = true
						require.True(t, envelop.System())
						require.True(t, system.Ref().Equals(envelop.Sender()))
					}
				}
				require.True(t, sawOriginal, "失败回调必须保留原始信封与系统标记")
				require.Empty(t, failed)
				if stop {
					require.Empty(t, accepted)
					require.Eventually(t, func() bool { return atomic.LoadInt32(&remoteContext.state) == killed }, time.Second, time.Millisecond)
					require.True(t, remoteContext.mailbox.IsPaused(), "组件终止不能依赖恢复普通邮箱")
					require.NoError(t, system.Stop(time.Second))
					return
				}
				peer, _, _ := newAdmissionSystem(t, peerTLS, 2, 3)
				ref, err := peer.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					if _, ok := ctx.Message().(vivid.ActorRef); ok {
						ctx.Reply(ctx.Message())
					}
				}))
				require.NoError(t, err)
				for range 12 {
					message, err := system.Ask(ref, system.Ref(), time.Second).Result()
					require.NoError(t, err, "失败释放后，名额必须能持续复用")
					require.True(t, system.Ref().Equals(message.(vivid.ActorRef)))
				}
			})
		}
	}
}

func TestSystem_RemotingAdmissionDroppedByKilledActor(t *testing.T) {
	system, failed, _ := newAdmissionSystem(t, nil, 1, 1)
	ref, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
	require.NoError(t, err)
	value, exists := system.actorContexts.Load(ref.GetPath())
	require.True(t, exists)
	ctx := value.(*Context)
	ctx.mailbox.Pause()
	target, err := system.CreateRef("127.0.0.1:1", "/dropped")
	require.NoError(t, err)
	// 将获准的远程信封固定在已停止 Actor 的普通队列中，验证通用死亡路径释放。
	original := mailbox.NewEnvelop(false, system.Ref(), target, system.Ref())
	remoteMailbox := remoting.NewMailbox(system.options.Context, system, ref, system, system.remotingAdmission)
	remoteMailbox.Enqueue(original)
	system.Kill(ref, false, "admission test")
	ctx.mailbox.Resume()
	require.Same(t, original, receiveRetryResult(t, failed))
	system.remotingAdmission.Stop()
	require.Empty(t, failed, "死亡路径和停止清理不得重复失败")
}

func TestSystem_RemotingAdmissionStopTimeout(t *testing.T) {
	system, failed, _ := newAdmissionSystem(t, nil, 1, 1)
	value, exists := system.actorContexts.Load(system.remotingRef.GetPath())
	require.True(t, exists)
	remoteMailbox := value.(*Context).mailbox
	remoteMailbox.Pause()
	system.mailbox.Pause()
	t.Cleanup(func() { remoteMailbox.Resume(); system.mailbox.Resume(); system.scheduler.Stop() })
	target, err := system.CreateRef("127.0.0.1:1", "/timeout")
	require.NoError(t, err)
	original := mailbox.NewEnvelop(false, system.Ref(), target, system.Ref())
	system.ResolveMailbox(target).Enqueue(original)
	// 根邮箱暂停使组件尚未收到终止消息；Stop 超时仍须释放准入索引。
	err = system.Stop(20 * time.Millisecond)
	require.ErrorIs(t, err, vivid.ErrorActorSystemStopFailed)
	require.Same(t, original, receiveRetryResult(t, failed))
	remoteMailbox.Resume()
	system.mailbox.Resume()
	receiveRetryResult(t, system.guardClosedSignal)
	require.Empty(t, failed, "迟到的组件终止不能重复处理原始信封")
}
