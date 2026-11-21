package actor_test

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/testkit"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func TestSystem_RemotingDialBoundAndRecovery(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, secure := range []bool{false, true} {
		for _, stop := range []bool{false, true} {
			t.Run(map[bool]string{false: "tcp", true: "tls"}[secure]+map[bool]string{false: "/release", true: "/stop"}[stop], func(t *testing.T) {
				var config *tls.Config
				if secure {
					config = ca.Config(t, true)
				}
				client := startRemotingTLSSystem(t, config, vivid.WithActorSystemRemotingOption(
					vivid.WithActorSystemRemotingMaxConcurrentDials(2),
					vivid.WithActorSystemRemotingMaxPendingDials(2),
					vivid.WithActorSystemRemotingReconnectLimit(0),
				))
				server := startRemotingTLSSystem(t, config)
				echo, err := server.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					if _, ok := ctx.Message().(vivid.ActorRef); ok {
						ctx.Reply(ctx.Message())
					}
				}))
				require.NoError(t, err)
				checkEcho := func() {
					message, err := client.Ask(echo, client.Ref(), time.Second).Result()
					require.NoError(t, err)
					require.True(t, client.Ref().Equals(message.(vivid.ActorRef)))
				}
				checkEcho()
				ready := make(chan struct{})
				rejected, dead := make(chan string, 8), make(chan string, 16)
				_, err = client.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch message := ctx.Message().(type) {
					case *vivid.OnLaunch:
						ctx.EventStream().Subscribe(ctx, ves.RemotingConnectionFailedEvent{})
						ctx.EventStream().Subscribe(ctx, ves.DeathLetterEvent{})
						close(ready)
					case ves.RemotingConnectionFailedEvent:
						if errors.Is(message.Error, vivid.ErrorRemotingDialQueueFull) {
							rejected <- message.Address
						}
					case ves.DeathLetterEvent:
						dead <- message.Envelope.Receiver().GetAddress()
					}
				}))
				require.NoError(t, err)
				receiveTellResult(t, ready)
				accepted := make(chan net.Conn, 8)
				var targets []vivid.ActorRef
				for range 6 {
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					require.NoError(t, err)
					t.Cleanup(func() { _ = listener.Close() })
					go func() {
						conn, err := listener.Accept()
						if err == nil {
							accepted <- conn // 不回复 TLS 或框架握手，确定占住执行名额。
						}
					}()
					ref, err := client.CreateRef(listener.Addr().String(), "/blocked")
					require.NoError(t, err)
					targets = append(targets, ref)
				}
				for _, ref := range targets {
					client.Tell(ref, client.Ref())
				}
				first, second := receiveTellResult(t, accepted), receiveTellResult(t, accepted)
				defer first.Close()
				defer second.Close()
				rejectedAddresses := map[string]bool{}
				for range 2 {
					address := receiveTellResult(t, rejected)
					require.False(t, rejectedAddresses[address])
					rejectedAddresses[address] = true
				}
				for range 2 {
					require.True(t, rejectedAddresses[receiveTellResult(t, dead)], "队列满的消息应转为死信")
				}
				select {
				case conn := <-accepted:
					_ = conn.Close()
					t.Fatal("并发拨号超过配置上限")
				case <-time.After(50 * time.Millisecond):
				}
				checkEcho() // 拨号饱和不影响已经建立的健康连接。
				if stop {
					require.NoError(t, client.Stop(time.Second))
					for _, conn := range []net.Conn{first, second} {
						require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
						_, err := io.Copy(io.Discard, conn)
						var timeout net.Error
						require.False(t, errors.As(err, &timeout) && timeout.Timeout(), "停止应关闭执行中的连接")
					}
					select {
					case conn := <-accepted:
						_ = conn.Close()
						t.Fatal("停止后等待地址仍然拨号")
					case <-time.After(50 * time.Millisecond):
					}
					return
				}
				// 每释放一个握手任务，只允许下一个等待地址开始。
				require.NoError(t, first.Close())
				third := receiveTellResult(t, accepted)
				defer third.Close()
				select {
				case conn := <-accepted:
					_ = conn.Close()
					t.Fatal("单个释放名额启动了多个任务")
				case <-time.After(50 * time.Millisecond):
				}
				require.NoError(t, second.Close())
				fourth := receiveTellResult(t, accepted)
				defer fourth.Close()
				require.NoError(t, third.Close())
				require.NoError(t, fourth.Close())
				for range 4 {
					require.False(t, rejectedAddresses[receiveTellResult(t, dead)])
				}
				checkEcho()
				// 名额清理后，新的地址也应能首次建连并完成业务往返。
				next := startRemotingTLSSystem(t, config)
				nextRef, err := next.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					if _, ok := ctx.Message().(vivid.ActorRef); ok {
						ctx.Reply(ctx.Message())
					}
				}))
				require.NoError(t, err)
				_, err = client.Ask(nextRef, client.Ref(), time.Second).Result()
				require.NoError(t, err)
			})
		}
	}
}
