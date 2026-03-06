package kvstore

import (
	"errors"
	"time"

	"github.com/kercylan98/vivid/experimental/internal/dram/internal/kvstore/table"
)

func New(tableSize uint64) *KVStore {
	store := &KVStore{
		tableSize: tableSize,
	}

	return store
}

type KVStore struct {
	tables    []*table.SlabTable
	spares    []*table.SlabTable
	tableSize uint64
}

func (k *KVStore) generateTable() {
	if len(k.spares) > 0 {
		reuseTable := k.spares[len(k.spares)-1]
		reuseTable.Reuse()
		k.tables = append(k.tables, reuseTable)
		k.spares = k.spares[:len(k.spares)-1]
		return
	}

	k.tables = append(k.tables, table.NewSlabTable(k.tableSize))
}

func (k *KVStore) Compaction() (err error) {
	const compactionThreshold = 0.4
	for i := 0; i < len(k.tables); i++ {
		t := k.tables[i]
		// 无效内存占比达到 compactionThreshold 则清理
		if float64(t.Invalid()) >= float64(t.Inuse())*compactionThreshold {
			if err = k.evictTable(t); err != nil {
				return err
			}
		}

		if len(k.tables) > 1 && t.Inuse() == 0 {
			k.spares = append(k.spares, t)
			k.tables = append(k.tables[:i], k.tables[i+1:]...)
			i--
		}
	}

	// 清理长期未使用的表
	const compactionTableIdleDuration = time.Minute * 10
	for i := 0; i < len(k.spares); i++ {
		t := k.spares[i]
		if time.Since(time.Unix(0, t.ResetAt())) > compactionTableIdleDuration {
			k.spares = append(k.spares[:i], k.spares[i+1:]...)
			i--
		}
	}

	return nil
}

func (k *KVStore) evictTable(t *table.SlabTable) error {
	const batchSize = 1024
	var deletedTotal int
	for hashKey, value := range t.Iter() {
		if err := k.Put(hashKey, value); err != nil {
			// 如果空间不足，扩容新表后等待下次重试
			if errors.Is(err, table.ErrorNotEnoughSpace) {
				k.generateTable()
				return nil
			}
			return err
		}

		// 如果写入成功，删除数据，避免未全面清理导致新表 key 被删除，查找到陈旧的数据
		// 如果表已使用内存为 0，则重置表
		if t.Delete(hashKey); t.Inuse() == 0 {
			t.Reset()
		}

		// 如果数据过多考虑分批处理，避免单次写入数据过多导致性能下降
		if deletedTotal++; deletedTotal >= batchSize {
			break
		}
	}
	return nil
}

func (k *KVStore) Put(hashKey uint64, value []byte) error {
	if uint64(len(value)) > k.tableSize {
		return table.ErrorNotEnoughSpace
	}

	if len(k.tables) == 0 {
		k.generateTable()
	}

	// 尝试将数据写入最后一个表，如果空间不足，则生成新的表
	for {
		lastTable := k.tables[len(k.tables)-1]
		err := lastTable.Put(hashKey, value)
		if errors.Is(err, table.ErrorNotEnoughSpace) {
			k.generateTable()
			continue
		} else if err != nil {
			return err
		}

		return nil
	}
}

func (k *KVStore) Get(hashKey uint64) ([]byte, error) {
	for i := len(k.tables) - 1; i >= 0; i-- {
		value, err := k.tables[i].Get(hashKey)
		if errors.Is(err, table.ErrorKeyNotFound) {
			continue
		} else if err != nil {
			return nil, err
		}
		return value, nil
	}

	return nil, table.ErrorKeyNotFound
}
