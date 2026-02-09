// 包 cluster 提供分布式系统协调原语，包括用于Gossip协议中因果排序的版本向量。
package cluster

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/kercylan98/vivid/internal/messages"
)

// 常量定义版本向量比较结果
const (
	// VersionEqual 表示两个向量在所有分量上完全相同
	VersionEqual VersionOrder = iota

	// VersionBefore 表示向量v严格发生在other之前
	// 即v的所有分量 <= other，且至少有一个分量 <
	VersionBefore

	// VersionAfter 表示向量v严格发生在other之后
	// 即v的所有分量 >= other，且至少有一个分量 >
	VersionAfter

	// VersionConcurrent 表示向量存在并发修改
	// 即存在某些分量v更大，某些分量other更大
	VersionConcurrent
)

// 配置限制，防止资源耗尽攻击
const (
	// maxVersionVectorEntries 限制每个向量跟踪的节点数量
	// 防止恶意大向量导致内存耗尽
	maxVersionVectorEntries = 65535

	// maxNodeAddressLength 限制节点标识符长度，防止内存攻击
	maxNodeAddressLength = 256

	// maxCounterValue 版本计数器最大值，防止溢出攻击
	maxCounterValue = 1<<63 - 1 // 9,223,372,036,854,775,807
)

// 验证错误定义
var (
	// ErrVectorTooLarge 表示向量超过配置的大小限制
	ErrVectorTooLarge = errors.New("版本向量条目数超过最大限制")

	// ErrInvalidNodeAddress 表示节点地址格式错误或过长
	ErrInvalidNodeAddress = errors.New("节点地址格式无效或长度超限")

	// ErrVersionOverflow 表示计数器溢出（实践中极不可能发生）
	ErrVersionOverflow = errors.New("版本计数器溢出")
)

// VersionOrder 表示两个版本向量之间的因果顺序关系
type VersionOrder int

// NodeCount 表示版本向量中单个节点的计数器
// 用于确定性序列化和日志记录
type NodeCount struct {
	Node  string
	Count uint64
}

// String 返回节点计数的可读字符串表示
func (nc NodeCount) String() string {
	return fmt.Sprintf("%s:%d", nc.Node, nc.Count)
}

// VersionVector 实现不可变的版本向量（向量时钟），用于
// 跟踪分布式系统中事件的部分顺序关系。
//
// 零值表示空向量（所有计数器为0）。
// 所有操作返回新实例，确保并发读取安全。
//
// 示例用法：
//
//	v1 := NewVersionVector()
//	v2 := v1.Increment("node-a")
//	v3 := v1.Increment("node-b")
//	order := v2.Compare(v3) // VersionConcurrent
type VersionVector struct {
	// m 存储节点到计数器值的映射
	// 使用稀疏存储，只存储有变化的节点
	m map[string]uint64

	// entries 缓存排序后的条目，避免重复排序
	// 当向量被修改时，此缓存会失效
	entries []NodeCount
	dirty   bool

	// sizeHint 用于预先分配map大小，优化内存分配
	sizeHint int
}

// NewVersionVector 创建新的空版本向量
// 返回的向量已初始化内部map，可直接使用
func NewVersionVector() VersionVector {
	return VersionVector{
		m:        make(map[string]uint64),
		entries:  nil,
		dirty:    false,
		sizeHint: 0,
	}
}

// NewVersionVectorWithCapacity 使用预分配容量创建版本向量
// capacity 参数提示向量可能包含的节点数量
// 在已知大致节点数时使用，可减少内存分配次数
func NewVersionVectorWithCapacity(capacity int) VersionVector {
	if capacity <= 0 {
		capacity = 4 // 默认初始容量
	}
	if capacity > maxVersionVectorEntries {
		capacity = maxVersionVectorEntries
	}

	return VersionVector{
		m:        make(map[string]uint64, capacity),
		entries:  nil,
		dirty:    false,
		sizeHint: capacity,
	}
}

// Clone 创建版本向量的深拷贝
// 返回的副本与原向量完全独立，修改副本不会影响原向量
func (v VersionVector) Clone() VersionVector {
	if v.m == nil {
		return NewVersionVector()
	}

	// 计算合适的容量
	capacity := len(v.m)
	if capacity < v.sizeHint {
		capacity = v.sizeHint
	}

	out := VersionVector{
		m:        make(map[string]uint64, capacity),
		entries:  nil,  // 不复制缓存，让后续使用重新生成
		dirty:    true, // 新向量需要重新计算缓存
		sizeHint: capacity,
	}

	// 复制map内容
	for node, count := range v.m {
		out.m[node] = count
	}

	return out
}

// Get 返回指定节点的计数器值
// 如果节点不存在或向量为零值，返回0
func (v VersionVector) Get(node string) uint64 {
	if v.m == nil {
		return 0
	}
	return v.m[node]
}

// Increment 递增指定节点的计数器值并返回新向量
//
// 参数：
//
//	node - 要递增的节点标识符，应为归一化的节点地址
//
// 返回：
//
//	新版本向量，原节点的计数器值加1
//
// 注意：
//   - 不修改原向量
//   - 如果计数器达到最大值，返回ErrVersionOverflow错误
func (v VersionVector) Increment(node string) (VersionVector, error) {
	// 验证节点地址
	if err := validateNodeAddress(node); err != nil {
		return VersionVector{}, err
	}

	// 创建副本
	out := v.Clone()

	// 检查并处理溢出
	current := out.m[node]
	if current >= maxCounterValue {
		return VersionVector{}, fmt.Errorf("%w: 节点 %s 计数器达到最大值 %d",
			ErrVersionOverflow, node, maxCounterValue)
	}

	// 递增计数器
	out.m[node] = current + 1
	out.dirty = true // 标记缓存已过期

	return out, nil
}

// MustIncrement Increment 的便捷版本，计数器溢出时panic
// 仅在确定不会溢出的场景使用
func (v VersionVector) MustIncrement(node string) VersionVector {
	result, err := v.Increment(node)
	if err != nil {
		panic(err)
	}
	return result
}

// Merge 合并两个版本向量，逐分量取最大值
//
// 参数：
//
//	other - 要合并的另一个版本向量
//
// 返回：
//
//	新版本向量，每个节点取两个向量中的最大值
//
// 时间复杂度：O(n+m)，其中n和m分别是两个向量的非零节点数
func (v VersionVector) Merge(other VersionVector) VersionVector {
	// 处理边界情况
	if len(other.m) == 0 {
		return v.Clone()
	}
	if len(v.m) == 0 {
		return other.Clone()
	}

	// 估算合并后的大小
	estimatedSize := len(v.m)
	for node := range other.m {
		if _, exists := v.m[node]; !exists {
			estimatedSize++
		}
	}

	// 确保不超过最大限制
	if estimatedSize > maxVersionVectorEntries {
		estimatedSize = maxVersionVectorEntries
	}

	// 创建新向量
	out := VersionVector{
		m:        make(map[string]uint64, estimatedSize),
		entries:  nil,
		dirty:    true,
		sizeHint: estimatedSize,
	}

	// 先复制当前向量的所有元素
	for node, count := range v.m {
		out.m[node] = count
	}

	// 合并另一个向量的元素，取较大值
	for node, otherCount := range other.m {
		currentCount, exists := out.m[node]
		if !exists || otherCount > currentCount {
			out.m[node] = otherCount
		}
	}

	return out
}

// Compare 比较两个版本向量的因果顺序
//
// 比较规则：
//   - 如果所有节点计数器相等，返回 VersionEqual
//   - 如果v所有节点 <= other，且至少一个节点 <，返回 VersionBefore
//   - 如果v所有节点 >= other，且至少一个节点 >，返回 VersionAfter
//   - 否则返回 VersionConcurrent
//
// 时间复杂度：O(min(n, m)) 平均情况，最坏情况 O(n+m)
func (v VersionVector) Compare(other VersionVector) VersionOrder {
	// 快速路径：两个都为空
	if len(v.m) == 0 && len(other.m) == 0 {
		return VersionEqual
	}

	var vLessOther, vGreaterOther bool

	// 遍历v的所有节点
	for node, va := range v.m {
		vb := other.Get(node)
		if va < vb {
			vLessOther = true
		} else if va > vb {
			vGreaterOther = true
		}

		// 提前返回：如果已确定并发，无需继续比较
		if vLessOther && vGreaterOther {
			return VersionConcurrent
		}
	}

	// 遍历other中v不包含的节点
	for node, vb := range other.m {
		if _, exists := v.m[node]; !exists {
			// v中该节点计数为0
			if 0 < vb { // va = 0
				vLessOther = true
			}
			// 0 > vb 不可能成立

			if vLessOther && vGreaterOther {
				return VersionConcurrent
			}
		}
	}

	// 根据比较结果返回对应顺序
	if vLessOther && !vGreaterOther {
		return VersionBefore
	}
	if !vLessOther && vGreaterOther {
		return VersionAfter
	}
	if !vLessOther && !vGreaterOther {
		// 所有节点相等
		return VersionEqual
	}

	// 理论上不会执行到这里
	return VersionConcurrent
}

// Equal 检查两个版本向量是否完全相等
// 内部调用 Compare 方法，检查是否返回 VersionEqual
func (v VersionVector) Equal(other VersionVector) bool {
	return v.Compare(other) == VersionEqual
}

// IsConcurrentWith 检查两个向量是否存在并发修改
// 是 Compare 返回 VersionConcurrent 的便捷方法
func (v VersionVector) IsConcurrentWith(other VersionVector) bool {
	return v.Compare(other) == VersionConcurrent
}

// HappensBefore 检查当前向量是否发生在另一个向量之前
// 是 Compare 返回 VersionBefore 的便捷方法
func (v VersionVector) HappensBefore(other VersionVector) bool {
	return v.Compare(other) == VersionBefore
}

// HappensAfter 检查当前向量是否发生在另一个向量之后
// 是 Compare 返回 VersionAfter 的便捷方法
func (v VersionVector) HappensAfter(other VersionVector) bool {
	return v.Compare(other) == VersionAfter
}

// Size 返回向量中非零节点的数量
func (v VersionVector) Size() int {
	if v.m == nil {
		return 0
	}
	return len(v.m)
}

// IsEmpty 检查向量是否为空（所有节点计数为0）
func (v VersionVector) IsEmpty() bool {
	return len(v.m) == 0
}

// Nodes 返回向量中所有节点的列表（无序）
func (v VersionVector) Nodes() []string {
	if len(v.m) == 0 {
		return nil
	}

	nodes := make([]string, 0, len(v.m))
	for node := range v.m {
		nodes = append(nodes, node)
	}
	return nodes
}

// SortedEntries 返回按节点地址排序的 (node, count) 列表
// 结果用于确定性序列化和日志记录
//
// 注意：返回的切片不应修改，后续向量修改可能使缓存失效
func (v VersionVector) SortedEntries() []NodeCount {
	if len(v.m) == 0 {
		return nil
	}

	// 如果缓存有效，直接返回
	if !v.dirty && v.entries != nil {
		return v.entries
	}

	// 重新计算缓存
	entries := make([]NodeCount, 0, len(v.m))
	for node, count := range v.m {
		entries = append(entries, NodeCount{Node: node, Count: count})
	}

	// 按节点地址排序，确保确定性
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Node < entries[j].Node
	})

	return entries
}

// String 返回版本向量的可读字符串表示
// 格式：{节点1:计数1, 节点2:计数2, ...}
func (v VersionVector) String() string {
	if len(v.m) == 0 {
		return "{}"
	}

	entries := v.SortedEntries()
	var sb strings.Builder
	sb.WriteString("{")

	for i, entry := range entries {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(entry.Node)
		sb.WriteString(":")
		fmt.Fprintf(&sb, "%d", entry.Count)
	}

	sb.WriteString("}")
	return sb.String()
}

// Prune 修剪版本向量，移除不在活跃节点列表中的条目；等价于 PruneWithMax(activeNodes, 0)。
func (v VersionVector) Prune(activeNodes []string) VersionVector {
	return v.PruneWithMax(activeNodes, 0)
}

// PruneWithMax 修剪版本向量为仅包含 activeNodes 中的节点；若 len(activeNodes)>maxEntries 且 maxEntries>0 则按节点名排序后只保留前 maxEntries 个。
// maxEntries<=0 时使用默认 maxVersionVectorEntries。
func (v VersionVector) PruneWithMax(activeNodes []string, maxEntries int) VersionVector {
	if len(v.m) == 0 || len(activeNodes) == 0 {
		return NewVersionVector()
	}
	limit := maxEntries
	if limit <= 0 {
		limit = maxVersionVectorEntries
	}
	if len(activeNodes) > limit {
		activeNodes = append([]string(nil), activeNodes...)
		sort.Strings(activeNodes)
		activeNodes = activeNodes[:limit]
	}
	activeSet := make(map[string]bool, len(activeNodes))
	for _, node := range activeNodes {
		activeSet[node] = true
	}
	out := VersionVector{
		m:        make(map[string]uint64, len(activeSet)),
		entries:  nil,
		dirty:    true,
		sizeHint: len(activeSet),
	}
	for node, count := range v.m {
		if activeSet[node] {
			out.m[node] = count
		}
	}
	return out
}

// ContainsNode 检查向量是否包含指定节点
// 注意：即使节点存在，其计数器也可能为0（这种情况下节点通常不会被包含在map中）
func (v VersionVector) ContainsNode(node string) bool {
	if v.m == nil {
		return false
	}
	_, exists := v.m[node]
	return exists
}

// MaxCounter 返回向量中所有计数器的最大值
// 用于监控和诊断
func (v VersionVector) MaxCounter() uint64 {
	var max uint64
	for _, count := range v.m {
		if count > max {
			max = count
		}
	}
	return max
}

// TotalCount 返回所有计数器之和
// 用于监控向量的大小增长
func (v VersionVector) TotalCount() uint64 {
	var total uint64
	for _, count := range v.m {
		total += count
	}
	return total
}

// WriteVersionVector 将版本向量序列化到写入器
//
// 序列化格式：
//
//	[4字节] 条目数 N (uint32)
//	重复 N 次：
//	  [变长] 节点地址 (字符串)
//	  [8字节] 计数器值 (uint64)
//
// 错误处理：
//
//	如果条目数超过 maxVersionVectorEntries，返回 ErrVectorTooLarge
func WriteVersionVector(w *messages.Writer, v VersionVector) error {
	entries := v.SortedEntries()

	// 检查大小限制
	if len(entries) > maxVersionVectorEntries {
		return fmt.Errorf("%w: 实际 %d，最大 %d",
			ErrVectorTooLarge, len(entries), maxVersionVectorEntries)
	}

	// 写入条目数
	w.WriteUint32(uint32(len(entries)))

	// 写入每个条目
	for _, entry := range entries {
		// 写入节点地址
		if err := validateNodeAddress(entry.Node); err != nil {
			return fmt.Errorf("序列化无效节点地址 %q: %w", entry.Node, err)
		}
		w.WriteString(entry.Node)

		// 写入计数器值
		w.WriteUint64(entry.Count)
	}

	return w.Err()
}

// ReadVersionVector 从读取器反序列化版本向量
//
// 参数：
//
//	r - 消息读取器
//
// 返回：
//
//	反序列化的版本向量，或错误
//
// 错误处理：
//
//	如果条目数超过 maxVersionVectorEntries，返回 ErrVectorTooLarge
//	如果节点地址无效，返回 ErrInvalidNodeAddress
//	如果计数器溢出，返回 ErrVersionOverflow
func ReadVersionVector(r *messages.Reader) (VersionVector, error) {
	// 读取条目数
	n, err := r.ReadUint32()
	if err != nil {
		return VersionVector{}, fmt.Errorf("读取条目数失败: %w", err)
	}

	// 验证条目数
	if n > maxVersionVectorEntries {
		return VersionVector{}, fmt.Errorf("%w: 期望 %d，最大 %d",
			ErrVectorTooLarge, n, maxVersionVectorEntries)
	}

	// 创建版本向量
	out := NewVersionVectorWithCapacity(int(n))

	// 读取每个条目
	for i := uint32(0); i < n; i++ {
		// 读取节点地址
		node, err := r.ReadString()
		if err != nil {
			return VersionVector{}, fmt.Errorf("读取节点地址失败: %w", err)
		}

		// 验证节点地址
		if err := validateNodeAddress(node); err != nil {
			return VersionVector{}, fmt.Errorf("无效节点地址 %q: %w", node, err)
		}

		// 读取计数器值
		count, err := r.ReadUint64()
		if err != nil {
			return VersionVector{}, fmt.Errorf("读取计数器失败: %w", err)
		}

		// 检查计数器溢出
		if count > maxCounterValue {
			return VersionVector{}, fmt.Errorf("%w: 节点 %s 计数器 %d 超过最大值 %d",
				ErrVersionOverflow, node, count, maxCounterValue)
		}

		out.m[node] = count
	}

	return out, nil
}

// validateNodeAddress 验证节点地址的合法性
func validateNodeAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("%w: 地址不能为空", ErrInvalidNodeAddress)
	}

	if len(addr) > maxNodeAddressLength {
		return fmt.Errorf("%w: 地址长度 %d 超过最大限制 %d",
			ErrInvalidNodeAddress, len(addr), maxNodeAddressLength)
	}

	// 可选：添加更多验证规则
	// 例如：检查地址格式、禁止特殊字符等

	return nil
}

// GobEncode 实现 encoding.GobEncoder，供 gob 序列化使用（如集成测试的 Codec）。
func (v VersionVector) GobEncode() ([]byte, error) {
	entries := v.SortedEntries()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(entries); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode 实现 encoding.GobDecoder，供 gob 反序列化使用。
func (v *VersionVector) GobDecode(data []byte) error {
	var entries []NodeCount
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&entries); err != nil {
		return err
	}
	v.m = make(map[string]uint64, len(entries))
	for _, e := range entries {
		v.m[e.Node] = e.Count
	}
	v.entries = entries
	v.dirty = false
	v.sizeHint = len(v.m)
	return nil
}

// Compact 压缩版本向量，移除计数器为0的条目
// 主要用于清理内部状态，通常不需要显式调用
func (v VersionVector) Compact() VersionVector {
	if len(v.m) == 0 {
		return v
	}

	// 统计非零条目
	nonZeroCount := 0
	for _, count := range v.m {
		if count > 0 {
			nonZeroCount++
		}
	}

	// 如果没有零值条目，直接返回
	if nonZeroCount == len(v.m) {
		return v
	}

	// 创建新向量，只包含非零条目
	out := VersionVector{
		m:        make(map[string]uint64, nonZeroCount),
		entries:  nil,
		dirty:    true,
		sizeHint: nonZeroCount,
	}

	for node, count := range v.m {
		if count > 0 {
			out.m[node] = count
		}
	}

	return out
}

// AtomicVersionVector 提供原子操作的版本向量包装器
// 用于需要在多个goroutine间安全更新版本向量的场景
type AtomicVersionVector struct {
	value atomic.Value // 存储 VersionVector
}

// NewAtomicVersionVector 创建新的原子版本向量
func NewAtomicVersionVector(initial VersionVector) *AtomicVersionVector {
	avv := &AtomicVersionVector{}
	if initial.m == nil {
		initial = NewVersionVector()
	}
	avv.value.Store(initial)
	return avv
}

// Load 原子加载当前版本向量
func (avv *AtomicVersionVector) Load() VersionVector {
	return avv.value.Load().(VersionVector)
}

// Store 原子存储新版本向量
func (avv *AtomicVersionVector) Store(v VersionVector) {
	avv.value.Store(v)
}

// CompareAndSwap 原子比较并交换版本向量
// 如果当前值等于old，则存储新值new
// 返回是否成功交换
func (avv *AtomicVersionVector) CompareAndSwap(old, new VersionVector) bool {
	// 注意：这里使用了类型断言，确保类型正确
	current := avv.value.Load().(VersionVector)
	if !current.Equal(old) {
		return false
	}
	return avv.value.CompareAndSwap(old, new)
}

// Increment 原子递增指定节点的计数器
// 如果递增成功，返回新向量，否则返回错误
func (avv *AtomicVersionVector) Increment(node string) (VersionVector, error) {
	for {
		current := avv.Load()
		newVec, err := current.Increment(node)
		if err != nil {
			return VersionVector{}, err
		}

		if avv.CompareAndSwap(current, newVec) {
			return newVec, nil
		}
		// 如果CAS失败，重试
	}
}
