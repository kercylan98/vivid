package remoting

import (
	"fmt"
	"net"
	"sync"

	"github.com/kercylan98/vivid/internal/messages"
)

// ConnectionPool 管理到特定远程地址的连接池
type ConnectionPool struct {
	mu                  sync.RWMutex
	advertiseAddr       string
	transport           Transport
	hash                *ConsistentHash
	connections         map[interface{}]Connection // 物理连接映射
	connectionCount     int
	clientAdvertiseAddr string // 客户端的广告地址，用于握手
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
	cp.mu.RLock()
	hashSize := cp.hash.Size()
	cp.mu.RUnlock()

	if hashSize == 0 {
		// 没有可用连接，需要创建新连接
		cp.mu.RUnlock()
		return cp.createConnection()
	}

	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// 使用一致性哈希选择连接
	node := cp.hash.Get(senderKey)
	if node == nil {
		// 哈希环为空，创建新连接
		cp.mu.RUnlock()
		return cp.createConnection()
	}

	conn, ok := node.(Connection)
	if !ok {
		// 节点不是连接，创建新连接
		cp.mu.RUnlock()
		return cp.createConnection()
	}

	return conn, nil
}

// createConnection 创建新连接并添加到哈希环
func (cp *ConnectionPool) createConnection() (Connection, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// 双重检查，可能其他goroutine已经创建了
	if cp.connectionCount > 0 {
		// 尝试从现有连接中选择一个
		for conn := range cp.connections {
			return conn.(Connection), nil
		}
	}

	addr, err := net.ResolveTCPAddr("tcp", cp.advertiseAddr)
	if err != nil {
		return nil, err
	}

	// 创建新连接
	conn, err := cp.transport.Dial(addr)
	if err != nil {
		return nil, err
	}

	// 执行握手，交换广告地址
	if err := cp.performHandshake(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake failed: %w", err)
	}

	// 添加到连接池
	cp.connections[conn] = conn
	cp.hash.Add(conn)
	cp.connectionCount++

	return conn, nil
}

// performHandshake 执行握手，交换广告地址
// 客户端先接收服务端的广告地址，然后发送自己的广告地址
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

	// 注意：这里我们接收到了服务端的真实广告地址，但连接池的 advertiseAddr
	// 可能已经是正确的（因为是通过目标地址创建的），所以这里不需要更新
	// 但如果需要，可以在这里更新 cp.advertiseAddr = serverAdvertiseAddr

	return nil
}

// AddConnection 添加一个连接到池中（用于服务端接收的连接）
func (cp *ConnectionPool) AddConnection(conn Connection) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// 检查连接是否已存在
	if _, exists := cp.connections[conn]; exists {
		return
	}

	cp.connections[conn] = conn
	cp.hash.Add(conn)
	cp.connectionCount++
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
	cp.connectionCount--

	// 关闭连接
	_ = cp.transport.Close(conn)
}

// Close 关闭连接池中的所有连接
func (cp *ConnectionPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	var firstErr error
	for conn := range cp.connections {
		if err := cp.transport.Close(conn.(Connection)); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	cp.connections = make(map[interface{}]Connection)
	cp.hash = NewConsistentHash(150)
	cp.connectionCount = 0

	return firstErr
}
