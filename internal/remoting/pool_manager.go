package remoting

import (
	"net"
	"sync"
)

// NewConnectionPoolManager 创建新的连接池管理器
func NewConnectionPoolManager(advertiseAddr string) *ConnectionPoolManager {
	return &ConnectionPoolManager{
		pools:               make(map[string]*ConnectionPool),
		clientAdvertiseAddr: advertiseAddr,
	}
}

// ConnectionPoolManager 管理多个连接池（每个远程地址一个）
type ConnectionPoolManager struct {
	mu                  sync.RWMutex
	pools               map[string]*ConnectionPool // key: address.String()
	clientAdvertiseAddr string                     // 客户端的广告地址，用于握手
}

// GetOrCreatePool 获取或创建指定地址的连接池
func (cpm *ConnectionPoolManager) GetOrCreatePool(advertiseAddr string) *ConnectionPool {
	cpm.mu.RLock()
	pool, exists := cpm.pools[advertiseAddr]
	cpm.mu.RUnlock()

	if exists {
		return pool
	}

	// 需要创建新池
	cpm.mu.Lock()
	defer cpm.mu.Unlock()

	// 双重检查
	if pool, exists := cpm.pools[advertiseAddr]; exists {
		return pool
	}

	transport := GetTransport()
	clientAddr := cpm.clientAdvertiseAddr
	pool = NewConnectionPool(advertiseAddr, transport, clientAddr)
	cpm.pools[advertiseAddr] = pool
	return pool
}

// GetPool 获取指定地址的连接池
func (cpm *ConnectionPoolManager) GetPool(advertiseAddr net.Addr) *ConnectionPool {
	key := advertiseAddr.String()

	cpm.mu.RLock()
	defer cpm.mu.RUnlock()

	return cpm.pools[key]
}

// RemovePool 移除指定地址的连接池
func (cpm *ConnectionPoolManager) RemovePool(advertiseAddr net.Addr) {
	key := advertiseAddr.String()

	cpm.mu.Lock()
	defer cpm.mu.Unlock()

	pool, exists := cpm.pools[key]
	if exists {
		_ = pool.Close()
		delete(cpm.pools, key)
	}
}

// Close 关闭所有连接池
func (cpm *ConnectionPoolManager) Close() error {
	cpm.mu.Lock()
	defer cpm.mu.Unlock()

	var firstErr error
	for _, pool := range cpm.pools {
		if err := pool.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	cpm.pools = make(map[string]*ConnectionPool)
	return firstErr
}
