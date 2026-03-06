package table

import "encoding/binary"

const (
	// 元数据字段定义
	valueLenSize = 4 // value 长度字段大小 (uint32)

	// 当前元数据总大小
	slabTableMetadataSize = valueLenSize
)

// NewSlabTable 创建一个指定内存大小的 SlabTable。
// 参数 size 表示分配的总字节数。
// 返回初始化后的 *SlabTable。
func NewSlabTable(size uint64) *SlabTable {
	return &SlabTable{
		memory:    make([]byte, size),
		allocated: size,
		offset:    0,
		inuse:     0,
		keyOffset: make(map[uint64]uint64),
	}
}

// SlabTable 结构体用于管理 slab 内存表，支持通过哈希键存储与检索 value。
// 采用预分配字节切片管理内存，并通过 keyOffset 快速定位每个键值对的偏移量。
type SlabTable struct {
	memory    []byte            // 表内存数据
	keyOffset map[uint64]uint64 // 键的偏移量映射（hashKey -> 偏移量）
	allocated uint64            // 分配的内存总大小
	offset    uint64            // 当前写入数据的偏移量
	inuse     uint64            // 已使用的字节数
	invalid   uint64            // 无效的字节数
}

// Put 将指定的 value 写入 slab 表，并根据 hashKey 存储其偏移量
// 记录格式：4字节value长度 (uint32, 大端) + value 数据
// 若剩余空间不足（元数据 + value），则返回 ErrorNotEnoughSpace 错误
func (t *SlabTable) Put(hashKey uint64, value []byte) error {
	if _, exists := t.keyOffset[hashKey]; exists {
		t.Delete(hashKey)
	}

	valLen := uint64(len(value))
	need := slabTableMetadataSize + valLen

	if t.offset+need > t.allocated {
		return ErrorNotEnoughSpace
	}

	offset := t.offset
	t.keyOffset[hashKey] = offset

	// 写入元数据：value长度
	binary.BigEndian.PutUint32(t.memory[offset:], uint32(valLen))

	// 写入value数据
	valueStart := offset + slabTableMetadataSize
	copy(t.memory[valueStart:], value)

	t.offset += need
	t.inuse += need
	return nil
}

// Get 根据 hashKey 获取对应的 value
// 若 hashKey 不存在，则返回 ErrorKeyNotFound 错误
func (t *SlabTable) Get(hashKey uint64) ([]byte, error) {
	offset, ok := t.keyOffset[hashKey]
	if !ok {
		return nil, ErrorKeyNotFound
	}

	// 读取元数据：value长度
	valLen := binary.BigEndian.Uint32(t.memory[offset:])
	start := offset + slabTableMetadataSize
	end := start + uint64(valLen)

	// 返回value副本
	val := make([]byte, valLen)
	copy(val, t.memory[start:end])
	return val, nil
}

// Delete 删除键（标记删除）
func (t *SlabTable) Delete(hashKey uint64) bool {
	offset, ok := t.keyOffset[hashKey]
	if !ok {
		return false
	}

	var invalid uint64

	// 计算无效的字节数
	valLen := binary.BigEndian.Uint32(t.memory[offset:])
	invalid += slabTableMetadataSize
	invalid += uint64(valLen)

	delete(t.keyOffset, hashKey)
	t.invalid += invalid
	t.inuse -= invalid
	return true
}

// Inuse 返回已使用的字节数
func (t *SlabTable) Inuse() uint64 {
	return t.inuse
}

// Invalid 返回无效的字节数
func (t *SlabTable) Invalid() uint64 {
	return t.invalid
}
