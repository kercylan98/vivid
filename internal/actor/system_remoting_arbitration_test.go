package actor_test

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/testkit"
	"github.com/stretchr/testify/require"
)

func TestSystem_RemotingSimultaneousFallback(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, mode := range []string{"plaintext", "tls", "mtls"} {
		for _, action := range []string{"preferred", "fallback", "stop"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				addresses := []string{clusterTestAddress(t), clusterTestAddress(t)}
				sort.Strings(addresses)
				listener, err := net.Listen("tcp", addresses[1])
				require.NoError(t, err)
				defer listener.Close()
				var config, peerConfig *tls.Config
				if mode != "plaintext" {
					config, peerConfig = ca.Config(t, mode == "mtls"), ca.Config(t, mode == "mtls")
					listener = tls.NewListener(listener, peerConfig)
				}
				system := startRemotingTLSSystem(t, config,
					vivid.WithActorSystemRemoting(addresses[0]),
					vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnectLimit(0)),
				)
				accepted := make(chan net.Conn, 1)
				go func() {
					conn, err := listener.Accept()
					if err == nil {
						accepted <- conn
					}
				}()
				target, err := system.CreateRef(addresses[1], "/arbitration")
				require.NoError(t, err)
				system.Tell(target, system.Ref())
				outbound := receiveTellResult(t, accepted)
				defer outbound.Close()
				request := &remoting.Handshake{}
				require.NoError(t, request.Wait(outbound))
				require.Equal(t, addresses[0], request.AdvertiseAddr)
				// 优选出站连接已发出请求，但暂不回复；反向连接应留到仲裁结果确定。
				connect := func() net.Conn {
					var conn net.Conn
					var err error
					if peerConfig == nil {
						conn, err = net.DialTimeout("tcp", addresses[0], time.Second)
					} else {
						conn, err = tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", addresses[0], peerConfig)
					}
					require.NoError(t, err)
					t.Cleanup(func() { _ = conn.Close() })
					require.NoError(t, (&remoting.Handshake{AdvertiseAddr: addresses[1]}).Send(conn))
					return conn
				}
				inbound := connect()
				require.NoError(t, inbound.SetReadDeadline(time.Now().Add(30*time.Millisecond)))
				_, err = inbound.Read(make([]byte, 1))
				var timeout net.Error
				require.ErrorAs(t, err, &timeout, "反向连接不能提前关闭或确认")
				require.True(t, timeout.Timeout())
				if action == "stop" {
					require.NoError(t, system.Stop(time.Second))
					for _, conn := range []net.Conn{inbound, outbound} {
						require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
						_, err := io.Copy(io.Discard, conn)
						if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
							var networkError net.Error
							require.ErrorAs(t, err, &networkError)
							require.False(t, networkError.Timeout(), "停止必须释放两条待仲裁连接")
						}
					}
					return
				}
				// 每个 Endpoint 只暂存一条反向连接，更多重复接入须及时关闭。
				duplicate := connect()
				require.NoError(t, duplicate.SetReadDeadline(time.Now().Add(time.Second)))
				_, err = duplicate.Read(make([]byte, 1))
				require.Error(t, err)
				if errors.As(err, &timeout) {
					require.False(t, timeout.Timeout(), "重复连接不能无限累积")
				}
				if action == "preferred" {
					require.NoError(t, (&remoting.Handshake{AdvertiseAddr: addresses[1]}).Send(outbound))
					require.NoError(t, outbound.SetReadDeadline(time.Now().Add(time.Second)))
					frame, err := remoting.ReadFrame(outbound, nil, remoting.FrameReadLimits{MaxDataLen: 4096})
					require.NoError(t, err)
					require.Equal(t, remoting.FrameCtrlData, frame.Type)
					require.NoError(t, inbound.SetReadDeadline(time.Now().Add(time.Second)))
					_, err = inbound.Read(make([]byte, 1))
					require.Error(t, err)
					if errors.As(err, &timeout) {
						require.False(t, timeout.Timeout(), "优选握手完成后必须释放备用连接")
					}
					return
				}
				require.NoError(t, outbound.Close())
				require.NoError(t, inbound.SetReadDeadline(time.Now().Add(time.Second)))
				response, err := remoting.ReadFrame(inbound, nil, remoting.FrameReadLimits{MaxDataLen: 4096})
				require.NoError(t, err, "优选拨号失败后应接管已有反向连接")
				require.Equal(t, remoting.FrameCtrlHandshake, response.Type)
				require.Equal(t, addresses[0], string(response.Data))
				frame, err := remoting.ReadFrame(inbound, nil, remoting.FrameReadLimits{MaxDataLen: 4096})
				require.NoError(t, err)
				require.Equal(t, remoting.FrameCtrlData, frame.Type, "零重试也应通过已有连接发送待发消息")
			})
		}
	}
}
