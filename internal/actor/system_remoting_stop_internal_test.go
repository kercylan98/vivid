package actor

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/testkit"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

type stopEnvelopHandler struct {
	remoting.NetworkEnvelopHandler
	onFail func()
}

func (h *stopEnvelopHandler) HandleFailedRemotingEnvelop(envelop vivid.Envelop) {
	h.NetworkEnvelopHandler.HandleFailedRemotingEnvelop(envelop)
	h.onFail()
}

func TestSystem_RemotingRebuildBeforeOldEndpointStops(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, mode := range []string{"plaintext", "tls", "mtls"} {
		t.Run(mode, func(t *testing.T) {
			var localConfig, peerConfig *tls.Config
			if mode != "plaintext" {
				localConfig, peerConfig = ca.Config(t, mode == "mtls"), ca.Config(t, mode == "mtls")
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			peerAddress := listener.Addr().String()
			if peerConfig != nil {
				listener = tls.NewListener(listener, peerConfig)
			}
			refused := make(chan struct{})
			go func() {
				defer close(refused)
				conn, err := listener.Accept()
				if err == nil {
					_ = conn.SetDeadline(time.Now().Add(time.Second))
					_ = new(remoting.Handshake).Wait(conn)
					_ = conn.Close()
				}
			}()
			local, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			address := local.Addr().String()
			require.NoError(t, local.Close())
			newSystem := func(address string, config *tls.Config) *System {
				return NewSystem(
					vivid.WithActorSystemRemoting(address),
					vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingTLSConfig(config), vivid.WithActorSystemRemotingReconnectLimit(0)),
					vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
				)
			}
			system := newSystem(address, localConfig)
			entered, resume := make(chan struct{}), make(chan struct{})
			var release, failure sync.Once
			defer func() { release.Do(func() { close(resume) }); require.NoError(t, system.Stop(time.Second)) }()
			system.remotingEnvelopHandler = &stopEnvelopHandler{NetworkEnvelopHandler: system.remotingEnvelopHandler, onFail: func() {
				// 在真实失败回调中固定旧端点尚未结束的窗口，新路由必须已经可用。
				failure.Do(func() { close(entered); <-resume })
			}}
			created, killed := make(chan vivid.ActorRef, 8), make(chan vivid.ActorRef, 8)
			system.eventStream = &retryEventStream{EventStream: system.eventStream, observe: func(_ vivid.EventStreamContext, event vivid.Message) {
				switch message := event.(type) {
				case ves.ActorSpawnedEvent:
					if message.Type.String() == "*remoting.endpoint" {
						created <- message.ActorRef
					}
				case ves.ActorKilledEvent:
					if message.Type.String() == "*remoting.endpoint" {
						killed <- message.ActorRef
					}
				}
			}}
			require.NoError(t, system.Start())
			target, err := system.CreateRef(peerAddress, "/retry")
			require.NoError(t, err)
			system.Tell(target, system.Ref())
			old := receiveRetryResult(t, created)
			receiveRetryResult(t, entered)
			receiveRetryResult(t, refused)
			require.NoError(t, listener.Close())
			peer := newSystem(peerAddress, peerConfig)
			require.NoError(t, peer.Start())
			defer func() { require.NoError(t, peer.Stop(time.Second)) }()
			_, err = peer.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if message, ok := ctx.Message().(vivid.ActorRef); ok {
					ctx.Reply(message)
				}
			}), vivid.WithActorName("retry"))
			require.NoError(t, err)
			for round := range 2 {
				message, err := system.CreateRef(address, fmt.Sprintf("/message/%d", round))
				require.NoError(t, err)
				result, err := system.Ask(target, message, time.Second).Result()
				require.NoError(t, err, "沿用业务引用的新消息应通过新端点完成请求")
				require.True(t, message.Equals(result.(vivid.ActorRef)))
				if round == 0 {
					next := receiveRetryResult(t, created)
					require.False(t, old.Equals(next))
					release.Do(func() { close(resume) })
					require.True(t, old.Equals(receiveRetryResult(t, killed)))
				}
			}
		})
	}
}
