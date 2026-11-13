package remoting

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid/internal/testkit"
	"github.com/stretchr/testify/require"
)

func TestTransport_TLSVerification(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, version := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		t.Run(tls.VersionName(version), func(t *testing.T) {
			config := ca.Config(t, true)
			config.MinVersion, config.MaxVersion = version, version
			for _, address := range []string{"localhost:1", "127.0.0.1:2", "other.invalid:3"} {
				t.Run(address, func(t *testing.T) {
					serverRaw, clientRaw := net.Pipe()
					defer serverRaw.Close()
					defer clientRaw.Close()
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					server := tls.Server(serverRaw, ca.Config(t, true))
					handshake := make(chan error, 1)
					go func() { handshake <- server.HandshakeContext(ctx) }()
					conn, err := handshakeTLS(ctx, clientRaw, address, config)
					if address == "other.invalid:3" {
						var hostname x509.HostnameError
						require.ErrorAs(t, err, &hostname, "每次连接必须按当前目标校验，不能复用上个目标名称")
						require.Error(t, <-handshake)
					} else {
						require.NoError(t, err)
						require.NoError(t, <-handshake)
						require.Equal(t, version, conn.(*tls.Conn).ConnectionState().Version)
					}
					require.Empty(t, config.ServerName)
				})
			}
		})
	}
}

func TestTransport_TLSCloseDoesNotWaitForPeer(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, name := range []string{"session", "pending"} {
		t.Run(name, func(t *testing.T) {
			serverRaw, clientRaw := net.Pipe()
			defer serverRaw.Close()
			defer clientRaw.Close()
			require.NoError(t, serverRaw.SetDeadline(time.Now().Add(time.Second)))
			require.NoError(t, clientRaw.SetDeadline(time.Now().Add(time.Second)))
			server := tls.Server(serverRaw, ca.Config(t, true))
			config := ca.Config(t, true)
			config.ServerName = "localhost"
			client := tls.Client(clientRaw, config)
			handshake := make(chan error, 1)
			go func() { handshake <- server.Handshake() }()
			require.NoError(t, client.Handshake())
			require.NoError(t, <-handshake)
			require.NoError(t, serverRaw.SetDeadline(time.Time{}))
			require.NoError(t, clientRaw.SetDeadline(time.Time{}))
			closed := make(chan struct{})
			go func() {
				if name == "session" {
					_ = newSession("peer", "local", server, sessionRoleInbound).Close()
				} else {
					manager := newEndpointManager()
					pending := manager.newPendingConnection(0)
					pending.attach(server)
					pending.close()
				}
				close(closed)
			}()
			// 对端不读取 close_notify，关闭仍应释放 TCP，而非等待 TLS 内部的 5 秒期限。
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("TLS 关闭被静默对端阻塞")
			}
		})
	}
}
