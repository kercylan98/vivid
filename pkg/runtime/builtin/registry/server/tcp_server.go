package server

import (
	"fmt"
	"net"
	"sync"

	"github.com/kercylan98/vivid/pkg/runtime/builtin/registry"
)

var _ registry.Server = (*TCPServer)(nil)

// NewTCPServer 创建一个新的 TCP 服务器。
// 提供完整的生命周期管理，需要调用 Start() 来启动。
func NewTCPServer(config *TCPServerConfiguration) *TCPServer {
	if config == nil {
		config = NewTCPServerConfiguration()
	}
	return &TCPServer{
		config:  *config,
		closeCh: make(chan struct{}),
	}
}

// NewTCPServerFromListener 从已有的 listener 创建 TCP 服务器。
// listener 的生命周期由外部管理，服务器不会关闭它。
func NewTCPServerFromListener(listener net.Listener, config *TCPServerConfiguration) *TCPServer {
	if config == nil {
		config = NewTCPServerConfiguration()
	}
	return &TCPServer{
		config:          *config,
		listener:        listener,
		externalManager: true,
		closeCh:         make(chan struct{}),
	}
}

// TCPServer 实现了 runtime.Server 接口的 TCP 服务器。
type TCPServer struct {
	config          TCPServerConfiguration
	listener        net.Listener
	externalManager bool // 是否外部管理 listener
	closeCh         chan struct{}
	closeOnce       sync.Once
	mu              sync.RWMutex
}

// Start 启动 TCP 服务器（仅当内部管理时有效）。
func (s *TCPServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果是外部管理的 listener，不需要启动
	if s.externalManager {
		return nil
	}

	// 如果已经启动，直接返回
	if s.listener != nil {
		return nil
	}

	listener, err := net.Listen(s.config.Network, s.config.BindAddress)
	if err != nil {
		return err
	}

	s.listener = listener
	return nil
}

// Stop 停止 TCP 服务器（仅当内部管理时有效）。
func (s *TCPServer) Stop() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.closeCh)

		s.mu.Lock()
		defer s.mu.Unlock()

		// 只有内部管理的 listener 才关闭
		if !s.externalManager && s.listener != nil {
			err = s.listener.Close()
			s.listener = nil
		}
	})
	return err
}

// Accept 实现 Server 接口。
func (s *TCPServer) Accept() (registry.Connection, error) {
	s.mu.RLock()
	listener := s.listener
	s.mu.RUnlock()

	if listener == nil {
		return nil, fmt.Errorf("server not started")
	}

	conn, err := listener.Accept()
	if err != nil {
		select {
		case <-s.closeCh:
			return nil, fmt.Errorf("server closed")
		default:
			return nil, err
		}
	}

	// 设置 TCP 连接参数
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if s.config.NoDelay {
			tcpConn.SetNoDelay(true)
		}
		if s.config.ReadBufferSize > 0 {
			tcpConn.SetReadBuffer(s.config.ReadBufferSize)
		}
		if s.config.WriteBufferSize > 0 {
			tcpConn.SetWriteBuffer(s.config.WriteBufferSize)
		}
	}

	return newTCPConnection(conn), nil
}

// Address 实现 Server 接口。
func (s *TCPServer) Address() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.BindAddress
}

// TCPServerConfiguration TCP 服务器配置。
type TCPServerConfiguration struct {
	Network         string // 网络类型: tcp, tcp4, tcp6
	BindAddress     string // 监听地址
	NoDelay         bool   // 禁用 Nagle 算法
	ReadBufferSize  int    // 读缓冲区大小
	WriteBufferSize int    // 写缓冲区大小
}

// NewTCPServerConfiguration 创建默认的 TCP 服务器配置。
func NewTCPServerConfiguration() *TCPServerConfiguration {
	return &TCPServerConfiguration{
		Network:         "tcp",
		BindAddress:     ":0",
		NoDelay:         true,
		ReadBufferSize:  64 * 1024, // 64KB
		WriteBufferSize: 64 * 1024, // 64KB
	}
}
