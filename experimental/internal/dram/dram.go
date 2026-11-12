package dram

import (
	"sync"

	"github.com/kercylan98/vivid/experimental/internal/dram/internal/kvstore"
)

// DRAM 分布式共享内存
type DRAM struct {
	mu            sync.RWMutex
	localNode     *gossip.Node
	memberList    *gossip.MemberList
	shards        map[uint64]*Shard
	localStore    *kvstore.KVStore
	shardCount    uint64
	replicaFactor int
}

// Config DRAM 配置
type Config struct {
	ShardCount    uint64
	ReplicaFactor int
	TableSize     uint64
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ShardCount:    128,
		ReplicaFactor: 3,
		TableSize:     1024 * 1024, // 1MB
	}
}

// NewDRAM 创建分布式共享内存实例
func NewDRAM(localNode *gossip.Node, config *Config) *DRAM {
	if config == nil {
		config = DefaultConfig()
	}

	dram := &DRAM{
		localNode:     localNode,
		shards:        make(map[uint64]*Shard),
		localStore:    kvstore.New(config.TableSize),
		shardCount:    config.ShardCount,
		replicaFactor: config.ReplicaFactor,
	}

	// 初始化分片
	for i := uint64(0); i < config.ShardCount; i++ {
		dram.shards[i] = NewShard(i)
	}

	return dram
}

// SetMemberList 设置成员列表（与 gossip 集成）
func (d *DRAM) SetMemberList(memberList *gossip.MemberList) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.memberList = memberList
	d.rebalanceShards()
}

// rebalanceShards 重新平衡分片
func (d *DRAM) rebalanceShards() {
	if d.memberList == nil {
		return
	}

	nodes := d.memberList.GetAliveNodes()
	if len(nodes) == 0 {
		return
	}

	for _, shard := range d.shards {
		// 计算该分片的副本节点
		replicas := d.selectReplicas(shard.ID, nodes)
		shard.Replicas = replicas

		// 如果当前节点是副本，设置为 Leader（简化逻辑，实际应使用 Raft 选举）
		if contains(replicas, d.localNode.ID) {
			if shard.Leader == "" || !contains(replicas, shard.Leader) {
				shard.Leader = d.localNode.ID
			}
		}
	}
}

// selectReplicas 为分片选择副本节点
func (d *DRAM) selectReplicas(shardID uint64, nodes []*gossip.Node) []string {
	if len(nodes) == 0 {
		return nil
	}

	// 简化的哈希选择逻辑
	replicas := make([]string, 0, d.replicaFactor)
	for i := 0; i < d.replicaFactor && i < len(nodes); i++ {
		idx := int((shardID + uint64(i)) % uint64(len(nodes)))
		replicas = append(replicas, nodes[idx].ID)
	}
	return replicas
}

// GetShard 获取指定分片
func (d *DRAM) GetShard(shardID uint64) *Shard {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.shards[shardID]
}

// GetShardByKey 根据键获取分片（哈希路由）
func (d *DRAM) GetShardByKey(hashKey uint64) *Shard {
	shardID := hashKey % d.shardCount
	return d.GetShard(shardID)
}

// Put 写入数据（本地存储 + 分布式副本）
func (d *DRAM) Put(hashKey uint64, value []byte) error {
	shard := d.GetShardByKey(hashKey)
	if shard == nil {
		return ErrShardNotFound
	}

	// 写入本地存储
	if err := d.localStore.Put(hashKey, value); err != nil {
		return err
	}

	// 更新分片数据
	shard.Put(hashKey, value)

	// TODO: 同步到其他副本节点（通过 Raft）
	return nil
}

// Get 读取数据
func (d *DRAM) Get(hashKey uint64) ([]byte, error) {
	// 优先从本地存储读取
	val, err := d.localStore.Get(hashKey)
	if err == nil {
		return val, nil
	}

	// 从分片数据读取
	shard := d.GetShardByKey(hashKey)
	if shard == nil {
		return nil, ErrShardNotFound
	}

	val, ok := shard.Get(hashKey)
	if !ok {
		return nil, ErrKeyNotFound
	}

	return val, nil
}

// Delete 删除数据
func (d *DRAM) Delete(hashKey uint64) error {
	shard := d.GetShardByKey(hashKey)
	if shard == nil {
		return ErrShardNotFound
	}

	shard.Delete(hashKey)
	// TODO: 同步到其他副本节点

	return nil
}

// GetLocalStore 获取本地存储（用于与底层 kvstore 集成）
func (d *DRAM) GetLocalStore() *kvstore.KVStore {
	return d.localStore
}

// Compaction 执行压缩
func (d *DRAM) Compaction() error {
	return d.localStore.Compaction()
}

// contains 检查字符串是否在切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
