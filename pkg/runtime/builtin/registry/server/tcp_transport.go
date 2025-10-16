package server

import (
	"net"
	"time"

	"github.com/kercylan98/vivid/pkg/runtime/builtin/registry"
)

var _ registry.Transport = (*tcpTransport)(nil)

// NewTCPTransport 创建一个新的 TCP 传输层。
func NewTCPTransport(config *TCPTransportConfiguration) registry.Transport {
	if config == nil {
		config = NewTCPTransportConfiguration()
	}
	return &tcpTransport{
		config: *config,
	}
}

// tcpTransport 实现了 runtime.Transport 接口。
type tcpTransport struct {
	config TCPTransportConfiguration
}

// Dial 实现 Transport 接口。
func (t *tcpTransport) Dial(address string) (registry.Connection, error) {
	dialer := &net.Dialer{
		Timeout:   t.config.DialTimeout,
		KeepAlive: t.config.KeepAlive,
	}

	conn, err := dialer.Dial(t.config.Network, address)
	if err != nil {
		return nil, err
	}

	// 设置 TCP 连接参数
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if t.config.NoDelay {
			tcpConn.SetNoDelay(true)
		}
		if t.config.ReadBufferSize > 0 {
			tcpConn.SetReadBuffer(t.config.ReadBufferSize)
		}
		if t.config.WriteBufferSize > 0 {
			tcpConn.SetWriteBuffer(t.config.WriteBufferSize)
		}
	}

	return newTCPConnection(conn), nil
}

// TCPTransportConfiguration TCP 传输层配置。
type TCPTransportConfiguration struct {
	Network         string        // 网络类型: tcp, tcp4, tcp6
	DialTimeout     time.Duration // 连接超时
	KeepAlive       time.Duration // 保活时间
	NoDelay         bool          // 禁用 Nagle 算法
	ReadBufferSize  int           // 读缓冲区大小
	WriteBufferSize int           // 写缓冲区大小
}

// NewTCPTransportConfiguration 创建默认的 TCP 传输层配置。
func NewTCPTransportConfiguration() *TCPTransportConfiguration {
	return &TCPTransportConfiguration{
		Network:         "tcp",
		DialTimeout:     time.Second * 10,
		KeepAlive:       time.Second * 30,
		NoDelay:         true,
		ReadBufferSize:  64 * 1024, // 64KB
		WriteBufferSize: 64 * 1024, // 64KB
	}
}
