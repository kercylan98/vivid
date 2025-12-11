package remoting

import (
	"fmt"
	"net"
	"sync"

	"github.com/kercylan98/vivid/internal/messages"
)

// ConnectionPool 管理到特定远程地址的连接池
type ConnectionPool struct {
	mu                  sync.Mutex
	advertiseAddr       string
	transport           Transport
	hash                *ConsistentHash
	connections         map[interface{}]Connection // 物理连接映射
	clientAdvertiseAddr string                     // 客户端的广告地址，用于握手
}

// NewConnectionPool 创建新的连接池
func NewConnectionPool(advertiseAddr string, transport Transport, clientAdvertiseAddr string) *ConnectionPool {
	return &ConnectionPool{
		advertiseAddr:       advertiseAddr,
		transport:           transport,
		hash:                NewConsistentHash(150), // 每个连接150个虚拟节点
		connections:         make(map[interface{}]Connection),
		clientAdvertiseAddr: clientAdvertiseAddr,
	}
}

// GetConnection 根据sender获取连接（使用一致性哈希）
func (cp *ConnectionPool) GetConnection(senderKey string) (Connection, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.hash.Size() == 0 {
		return cp.createConnectionLocked()
	}

	node := cp.hash.Get(senderKey)
	if node == nil {
		return cp.createConnectionLocked()
	}

	conn, ok := node.(Connection)
	if !ok {
		return cp.createConnectionLocked()
	}

	return conn, nil
}

// createConnectionLocked 创建新连接并添加到哈希环（锁已持有）
func (cp *ConnectionPool) createConnectionLocked() (Connection, error) {
	// 创建前双重检查
	if len(cp.connections) > 0 {
		for _, conn := range cp.connections {
			return conn, nil
		}
	}

	addr, err := net.ResolveTCPAddr("tcp", cp.advertiseAddr)
	if err != nil {
		return nil, err
	}

	conn, err := cp.transport.Dial(addr)
	if err != nil {
		return nil, err
	}

	// 在持有锁时进行握手并不优雅，但代码沿用原始逻辑，仅重构锁粒度
	err = cp.performHandshake(conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake failed: %w", err)
	}

	cp.connections[conn] = conn
	cp.hash.Add(conn)
	return conn, nil
}

// performHandshake 执行握手，交换广告地址
func (cp *ConnectionPool) performHandshake(conn Connection) error {
	// 先接收服务端的广告地址
	data, err := cp.transport.Receive(conn)
	if err != nil {
		return fmt.Errorf("failed to receive handshake: %w", err)
	}

	reader := messages.NewReader(data)
	var serverAdvertiseAddr string
	if err := reader.Read(&serverAdvertiseAddr); err != nil {
		return fmt.Errorf("failed to read handshake address: %w", err)
	}

	// 发送客户端的广告地址
	writer := messages.NewWriter()
	writer.WriteString(cp.clientAdvertiseAddr)
	handshakeData := writer.Bytes()
	if err := cp.transport.Send(conn, handshakeData); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	return nil
}

// AddConnection 添加一个连接到池中（用于服务端接收的连接）
func (cp *ConnectionPool) AddConnection(conn Connection) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, exists := cp.connections[conn]; exists {
		return
	}

	cp.connections[conn] = conn
	cp.hash.Add(conn)
}

// RemoveConnection 从池中移除连接
func (cp *ConnectionPool) RemoveConnection(conn Connection) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, exists := cp.connections[conn]; !exists {
		return
	}

	delete(cp.connections, conn)
	cp.hash.Remove(conn)
	_ = cp.transport.Close(conn)
}

// Close 关闭连接池中的所有连接
func (cp *ConnectionPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	var firstErr error
	for _, conn := range cp.connections {
		if err := cp.transport.Close(conn); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	cp.connections = make(map[interface{}]Connection)
	cp.hash = NewConsistentHash(150)

	return firstErr
}
