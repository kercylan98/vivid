package remoting

import (
	"crypto/sha256"
	"hash"
	"sort"
	"sync"
)

// ConsistentHash 实现基于虚拟节点的一致性哈希
type ConsistentHash struct {
	mu            sync.RWMutex
	hashFunc      hash.Hash
	virtualNodes  int                    // 每个物理节点的虚拟节点数
	ring          []uint32               // 哈希环，存储虚拟节点的哈希值
	nodeMap       map[uint32]interface{} // 哈希值到节点的映射
	virtualToReal map[uint32]interface{} // 虚拟节点到物理节点的映射
}

// NewConsistentHash 创建新的一致性哈希实例
func NewConsistentHash(virtualNodes int) *ConsistentHash {
	if virtualNodes <= 0 {
		virtualNodes = 150 // 默认值
	}
	return &ConsistentHash{
		hashFunc:      sha256.New(),
		virtualNodes:  virtualNodes,
		ring:          make([]uint32, 0),
		nodeMap:       make(map[uint32]interface{}),
		virtualToReal: make(map[uint32]interface{}),
	}
}

// Add 添加一个节点到哈希环
func (ch *ConsistentHash) Add(node interface{}) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// 为每个物理节点创建多个虚拟节点
	for i := 0; i < ch.virtualNodes; i++ {
		hash := ch.hashKey(nodeKey(node, i))
		ch.ring = append(ch.ring, hash)
		ch.nodeMap[hash] = node
		ch.virtualToReal[hash] = node
	}

	// 对哈希环排序，便于查找
	sort.Slice(ch.ring, func(i, j int) bool {
		return ch.ring[i] < ch.ring[j]
	})
}

// Remove 从哈希环移除一个节点
func (ch *ConsistentHash) Remove(node interface{}) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// 移除该节点的所有虚拟节点
	newRing := make([]uint32, 0, len(ch.ring))
	for _, hash := range ch.ring {
		if ch.virtualToReal[hash] != node {
			newRing = append(newRing, hash)
		} else {
			delete(ch.nodeMap, hash)
			delete(ch.virtualToReal, hash)
		}
	}
	ch.ring = newRing
}

// Get 根据key获取对应的节点
func (ch *ConsistentHash) Get(key string) interface{} {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.ring) == 0 {
		return nil
	}

	hash := ch.hashKey(key)

	// 在哈希环上查找第一个大于等于该哈希值的节点
	idx := sort.Search(len(ch.ring), func(i int) bool {
		return ch.ring[i] >= hash
	})

	// 如果没找到，则使用第一个节点（环形结构）
	if idx == len(ch.ring) {
		idx = 0
	}

	return ch.nodeMap[ch.ring[idx]]
}

// Size 返回当前节点数量（物理节点数）
func (ch *ConsistentHash) Size() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if ch.virtualNodes == 0 {
		return 0
	}
	return len(ch.ring) / ch.virtualNodes
}

// hashKey 计算key的哈希值
func (ch *ConsistentHash) hashKey(key string) uint32 {
	ch.hashFunc.Reset()
	ch.hashFunc.Write([]byte(key))
	hashBytes := ch.hashFunc.Sum(nil)
	// 取前4字节作为uint32
	return uint32(hashBytes[0])<<24 | uint32(hashBytes[1])<<16 | uint32(hashBytes[2])<<8 | uint32(hashBytes[3])
}

// nodeKey 生成节点的key（用于虚拟节点）
func nodeKey(node interface{}, virtualIndex int) string {
	// 使用节点地址和虚拟索引生成唯一key
	return nodeString(node) + "#" + string(rune(virtualIndex))
}

// nodeString 将节点转换为字符串表示
func nodeString(node interface{}) string {
	// 尝试使用String方法
	if s, ok := node.(interface{ String() string }); ok {
		return s.String()
	}
	// 尝试使用Connection接口
	if conn, ok := node.(Connection); ok {
		return conn.RemoteAddr().String() + "@" + conn.LocalAddr().String()
	}
	return ""
}

