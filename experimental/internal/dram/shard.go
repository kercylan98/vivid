package dram

import (
	"sync"

	"github.com/google/uuid"
)

// Shard 分布式共享内存的分片
type Shard struct {
	ID       uint64
	Replicas []string // 副本节点 ID 列表
	Leader   string   // 主节点 ID
	Version  uint64
	Data     map[uint64][]byte
	mu       sync.RWMutex
}

// NewShard 创建新分片
func NewShard(id uint64) *Shard {
	return &Shard{
		ID:       id,
		Replicas: make([]string, 0),
		Data:     make(map[uint64][]byte),
		Version:  1,
	}
}

// Put 写入数据
func (s *Shard) Put(key uint64, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = value
	s.Version++
	return nil
}

// Get 读取数据
func (s *Shard) Get(key uint64) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Data[key]
	return val, ok
}

// Delete 删除数据
func (s *Shard) Delete(key uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Data[key]; ok {
		delete(s.Data, key)
		s.Version++
		return true
	}
	return false
}

// GetVersion 获取版本号
func (s *Shard) GetVersion() uint64 {
	return s.Version
}

// SetLeader 设置主节点
func (s *Shard) SetLeader(leaderID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Leader = leaderID
}

// GetLeader 获取主节点
func (s *Shard) GetLeader() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Leader
}
