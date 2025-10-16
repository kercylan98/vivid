package registry

import (
	"fmt"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

// newConnectionPool 创建一个新的连接池。
func newConnectionPool(advertiseAddress string, config *ConnectionPoolConfiguration) *connectionPool {
	return &connectionPool{
		advertiseAddress: advertiseAddress,
		config:           *config,
		slots:            xsync.NewMapOf[string, *connectionSlot](),
		closed:           make(chan struct{}),
	}
}

// connectionPool 管理到各个远程地址的连接。
type connectionPool struct {
	advertiseAddress string
	config           ConnectionPoolConfiguration
	slots            *xsync.MapOf[string, *connectionSlot]
	closed           chan struct{}
	closeOnce        sync.Once
}

// connectionSlot 表示到某个地址的连接槽。
type connectionSlot struct {
	address    string
	pool       *connectionPool
	conn       Connection
	sendQueue  chan *Packet
	mu         sync.Mutex
	closed     bool
	lastActive time.Time
}

// Send 发送数据包到指定地址。
func (p *connectionPool) Send(address string, packet *Packet) error {
	select {
	case <-p.closed:
		return fmt.Errorf("connection pool closed")
	default:
	}

	slot, err := p.getOrCreateSlot(address)
	if err != nil {
		return err
	}

	select {
	case slot.sendQueue <- packet:
		return nil
	case <-p.closed:
		return fmt.Errorf("connection pool closed")
	default:
		return fmt.Errorf("send queue full for address: %s", address)
	}
}

// Get 获取到指定地址的连接。
func (p *connectionPool) Get(address string) (Connection, error) {
	select {
	case <-p.closed:
		return nil, fmt.Errorf("connection pool closed")
	default:
	}

	slot, err := p.getOrCreateSlot(address)
	if err != nil {
		return nil, err
	}

	return slot.conn, nil
}

// Remove 移除到指定地址的连接。
func (p *connectionPool) Remove(address string) {
	if slot, ok := p.slots.LoadAndDelete(address); ok {
		slot.close()
	}
}

// Close 关闭连接池及所有连接。
func (p *connectionPool) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		close(p.closed)

		// 关闭所有连接槽
		p.slots.Range(func(address string, slot *connectionSlot) bool {
			slot.close()
			return true
		})
		p.slots.Clear()
	})
	return closeErr
}

// getOrCreateSlot 获取或创建到指定地址的连接槽。
func (p *connectionPool) getOrCreateSlot(address string) (*connectionSlot, error) {
	// 尝试加载现有连接槽
	if slot, ok := p.slots.Load(address); ok {
		slot.mu.Lock()
		if !slot.closed {
			slot.lastActive = time.Now()
			slot.mu.Unlock()
			return slot, nil
		}
		slot.mu.Unlock()
	}

	// 需要创建新连接槽
	if p.config.Transport == nil {
		return nil, fmt.Errorf("transport not configured")
	}

	// 建立连接
	conn, err := p.config.Transport.Dial(address)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	// 执行握手
	if err := p.performHandshake(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake failed with %s: %w", address, err)
	}

	// 创建连接槽
	slot := &connectionSlot{
		address:    address,
		pool:       p,
		conn:       conn,
		sendQueue:  make(chan *Packet, p.config.SendQueueSize),
		lastActive: time.Now(),
	}

	// 存储连接槽
	actual, loaded := p.slots.LoadOrStore(address, slot)
	if loaded {
		// 其他 goroutine 已经创建了连接，关闭当前连接
		slot.close()
		return actual, nil
	}

	// 启动发送 goroutine
	go slot.sendLoop()

	return slot, nil
}

// performHandshake 执行握手协议。
func (p *connectionPool) performHandshake(conn Connection) error {
	handshake := NewHandshake(p.advertiseAddress)
	data, err := handshake.Encode()
	if err != nil {
		return err
	}

	if err := conn.Send(data); err != nil {
		return err
	}

	// 等待握手响应（带超时）
	responseChan := make(chan []byte, 1)
	errChan := make(chan error, 1)

	go func() {
		resp, err := conn.Recv()
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- resp
	}()

	select {
	case resp := <-responseChan:
		responseHandshake := &Handshake{}
		if err := responseHandshake.Decode(resp); err != nil {
			return fmt.Errorf("decode response handshake failed: %w", err)
		}
		return nil
	case err := <-errChan:
		return err
	case <-time.After(p.config.HandshakeTimeout):
		return fmt.Errorf("handshake timeout")
	}
}

// sendLoop 处理发送队列。
func (slot *connectionSlot) sendLoop() {
	for {
		select {
		case <-slot.pool.closed:
			return
		case packet := <-slot.sendQueue:
			if err := slot.sendPacket(packet); err != nil {
				// 发送失败，尝试重连
				slot.handleSendError(err, packet)
			}
			ReleasePacket(packet)
		}
	}
}

// sendPacket 发送单个数据包。
func (slot *connectionSlot) sendPacket(packet *Packet) error {
	slot.mu.Lock()
	defer slot.mu.Unlock()

	if slot.closed {
		return fmt.Errorf("connection closed")
	}

	data, err := packet.Encode()
	if err != nil {
		return err
	}

	if err := slot.conn.Send(data); err != nil {
		return err
	}

	slot.lastActive = time.Now()
	return nil
}

// handleSendError 处理发送错误。
func (slot *connectionSlot) handleSendError(err error, packet *Packet) {
	// 简单策略：记录错误并移除连接
	// 后续发送会触发重连
	slot.pool.Remove(slot.address)
}

// close 关闭连接槽。
func (slot *connectionSlot) close() {
	slot.mu.Lock()
	defer slot.mu.Unlock()

	if slot.closed {
		return
	}

	slot.closed = true
	close(slot.sendQueue)

	if slot.conn != nil {
		slot.conn.Close()
	}
}
