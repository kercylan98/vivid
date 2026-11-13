package remoting

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

func handshakeTLS(ctx context.Context, conn net.Conn, address string, config *tls.Config) (net.Conn, error) {
	clientConfig := config.Clone()
	if clientConfig.ServerName == "" {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		clientConfig.ServerName = host
	}
	secured := tls.Client(conn, clientConfig)
	handshakeCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	if err := secured.HandshakeContext(handshakeCtx); err != nil {
		return nil, err
	}
	return secured, nil
}

func closeConnection(conn net.Conn) error {
	// Vivid 关闭帧由会话限时发送；不再等待 TLS close_notify，避免其重设写期限。
	if secured, ok := conn.(*tls.Conn); ok {
		conn = secured.NetConn()
	}
	_ = conn.SetDeadline(time.Now())
	return conn.Close()
}
