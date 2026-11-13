package actor_test

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/testkit"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func TestSystem_RemotingRejectsHandshakeAddress(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, mode := range []string{"plaintext", "tls", "mtls"} {
		t.Run(mode, func(t *testing.T) {
			var serverConfig, clientConfig *tls.Config
			if mode != "plaintext" {
				serverConfig, clientConfig = ca.Config(t, mode == "mtls"), ca.Config(t, mode == "mtls")
			}
			server := startRemotingTLSSystem(t, serverConfig)
			addresses := []string{"", "node.example", "0.0.0.0:8080", "[::]:8080", "[0:0:0:0:0:0:0:0]:8080", "[::ffff:0.0.0.0]:8080", "node:+80", "node:65536", " node:8080 "}
			// 连续拒绝超过 64 个请求，验证异常握手不会耗尽接入名额。
			for attempt := range 72 {
				func() {
					var conn net.Conn
					var err error
					dialer := &net.Dialer{Timeout: time.Second}
					if clientConfig == nil {
						conn, err = dialer.Dial("tcp", server.Ref().GetAddress())
					} else {
						conn, err = tls.DialWithDialer(dialer, "tcp", server.Ref().GetAddress(), clientConfig)
					}
					require.NoError(t, err)
					defer conn.Close()
					require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
					_, err = remoting.NewHandshakeFrame(addresses[attempt%len(addresses)]).WriteTo(conn)
					require.NoError(t, err)
					_, err = remoting.ReadFrame(conn, nil, remoting.FrameReadLimits{MaxControlLen: 4096, MaxDataLen: 1024})
					require.Error(t, err, "异常声明不能获得握手确认，attempt=%d", attempt)
					if networkError, ok := err.(net.Error); ok {
						require.False(t, networkError.Timeout(), "拒绝握手须立即关闭连接，不能等待超时回收")
					}
				}()
			}
			peer := startRemotingTLSSystem(t, clientConfig)
			ref, err := server.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if message, ok := ctx.Message().(vivid.ActorRef); ok {
					ctx.Reply(message)
				}
			}))
			require.NoError(t, err)
			message, err := peer.Ask(ref, peer.Ref(), time.Second).Result()
			require.NoError(t, err, "连续拒绝异常声明后仍能建立合法会话并应答")
			require.True(t, peer.Ref().Equals(message.(vivid.ActorRef)))
		})
	}
}

func TestSystem_RemotingRejectsHandshakeResponse(t *testing.T) {
	for _, address := range []string{"", "node.example", "0.0.0.0:8080", "[::]:8080", " node:8080 ", "other:8080"} {
		t.Run(address, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			require.NoError(t, listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second)))
			system := startRemotingTLSSystem(t, nil, vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnectLimit(0)))
			target, err := system.CreateRef(listener.Addr().String(), "/handshake")
			require.NoError(t, err)
			ready := make(chan struct{})
			failed := make(chan error, 4)
			_, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch message := ctx.Message().(type) {
				case *vivid.OnLaunch:
					ctx.EventStream().Subscribe(ctx, ves.RemotingConnectionFailedEvent{})
					close(ready)
				case ves.RemotingConnectionFailedEvent:
					if message.Address == target.GetAddress() {
						failed <- message.Error
					}
				}
			}))
			require.NoError(t, err)
			receiveTellResult(t, ready)
			system.Tell(target, system.Ref())
			conn, err := listener.Accept()
			require.NoError(t, err)
			defer conn.Close()
			require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
			request := &remoting.Handshake{}
			require.NoError(t, request.Wait(conn))
			require.Equal(t, system.Ref().GetAddress(), request.AdvertiseAddr)
			_, err = remoting.NewHandshakeFrame(address).WriteTo(conn)
			require.NoError(t, err)
			_, err = remoting.ReadFrame(conn, nil, remoting.FrameReadLimits{MaxControlLen: 4096, MaxDataLen: 1024})
			require.Error(t, err, "握手响应未通过校验时不能发送业务帧")
			if networkError, ok := err.(net.Error); ok {
				require.False(t, networkError.Timeout(), "拒绝响应后须关闭出站连接")
			}
			require.ErrorIs(t, receiveTellResult(t, failed), vivid.ErrorRemotingHandshake)
		})
	}
}
