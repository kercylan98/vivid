package remoting

import (
	"fmt"
	"net"
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/internal/remoting/serialize"
)

// Server 管理TCP服务器
type Server struct {
	mu            sync.RWMutex
	bindAddr      string
	advertiseAddr string
	transport     Transport
	listener      Listener
	poolManager   *ConnectionPoolManager
	handler       vivid.EnvelopHandler
	stopChan      chan struct{}
	wg            sync.WaitGroup
	provider      vivid.EnvelopProvider
}

// NewServer 创建新的服务器
func NewServer(
	bindAddr string,
	advertiseAddr string,
	poolManager *ConnectionPoolManager,
	handler vivid.EnvelopHandler,
	provider vivid.EnvelopProvider,
) *Server {
	return &Server{
		bindAddr:      bindAddr,
		advertiseAddr: advertiseAddr,
		transport:     GetTransport(),
		poolManager:   poolManager,
		handler:       handler,
		stopChan:      make(chan struct{}),
		provider:      provider,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return nil // 已经启动
	}

	addr, err := net.ResolveTCPAddr("tcp", s.bindAddr)
	if err != nil {
		return err
	}

	listener, err := s.transport.Listen(addr)
	if err != nil {
		return err
	}

	s.listener = listener

	// 启动接收循环
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return nil
	}

	close(s.stopChan)
	err := s.listener.Close()
	s.listener = nil

	s.wg.Wait()
	return err
}

// acceptLoop 接受连接的循环
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				// 监听器关闭
				return
			}

			// 执行握手，交换广告地址
			remoteAdvertiseAddr, err := s.performHandshake(conn)
			if err != nil {
				// 握手失败，关闭连接
				_ = conn.Close()
				continue
			}

			// 根据握手得到的广告地址获取或创建连接池
			pool := s.poolManager.GetOrCreatePool(remoteAdvertiseAddr)

			// 将连接添加到池中
			pool.AddConnection(conn)

			// 启动goroutine接收消息
			s.wg.Add(1)
			go s.receiveLoop(conn, pool)
		}
	}
}

// performHandshake 执行握手，交换广告地址
// 返回远程节点的广告地址
func (s *Server) performHandshake(conn Connection) (string, error) {
	// 先发送自己的广告地址
	writer := messages.NewWriter()
	writer.WriteString(s.advertiseAddr)
	handshakeData := writer.Bytes()
	if err := s.transport.Send(conn, handshakeData); err != nil {
		return "", fmt.Errorf("failed to send handshake: %w", err)
	}

	// 接收远程节点的广告地址
	data, err := s.transport.Receive(conn)
	if err != nil {
		return "", fmt.Errorf("failed to receive handshake: %w", err)
	}

	reader := messages.NewReader(data)
	var remoteAdvertiseAddr string
	if err := reader.Read(&remoteAdvertiseAddr); err != nil {
		return "", fmt.Errorf("failed to read handshake address: %w", err)
	}

	return remoteAdvertiseAddr, nil
}

// receiveLoop 从连接接收消息的循环
func (s *Server) receiveLoop(conn Connection, pool *ConnectionPool) {
	defer s.wg.Done()
	defer pool.RemoveConnection(conn)

	for {
		select {
		case <-s.stopChan:
			return
		default:
			// 从连接接收数据
			data, err := s.transport.Receive(conn)
			if err != nil {
				// 连接关闭或错误
				return
			}

			// 反序列化并处理消息
			message, err := serialize.DeserializeEnvelopWithRemoting(data, s.provider)
			if err != nil {
				// 反序列化失败，记录错误但继续处理
				continue
			}
			s.handler.HandleEnvelop(message)
		}
	}
}
