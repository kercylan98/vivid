package clusterstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshot_Replay(t *testing.T) {
	commands := []Command{
		{Operation: Put, Key: "singleton/orders", Value: []byte("node-a")},
		{Operation: Put, Key: "singleton/payments", Value: []byte("node-b")},
		{Operation: Put, Key: "singleton/orders", Value: []byte("node-c")}, // 比较失败
		{},
		{Operation: Delete, Key: "singleton/orders", ExpectedRevision: 1},
		{Operation: Put, Key: "singleton/orders", Value: []byte("node-c")},
		{Operation: Delete, Key: "singleton/orders", ExpectedRevision: 1}, // 旧归属不能删除新记录
		{Operation: Put, Key: "singleton/payments", ExpectedRevision: 2},
	}
	var first, second Store
	for _, command := range commands {
		require.Equal(t, apply(t, &first, command), apply(t, &second, command))
		require.Equal(t, first.Snapshot(), second.Snapshot(), "相同日志必须产生相同快照")
	}
	for prefix := 0; prefix <= len(commands); prefix++ {
		var source Store
		for _, command := range commands[:prefix] {
			apply(t, &source, command)
		}
		restored, err := Restore(source.Snapshot())
		require.NoError(t, err)
		require.Equal(t, source.Snapshot(), restored.Snapshot())
		for _, command := range commands[prefix:] {
			require.Equal(t, apply(t, &source, command), apply(t, restored, command))
		}
		require.Equal(t, first.Snapshot(), restored.Snapshot(), "恢复后重放后缀不能改变最终结果")
	}
}

func TestSnapshot_Independent(t *testing.T) {
	var store Store
	apply(t, &store, Command{Operation: Put, Key: "key", Value: []byte("value")})
	data := store.Snapshot()
	expected := bytes.Clone(data)
	history := store.Snapshot()
	restored, err := Restore(data)
	require.NoError(t, err)
	clear(data)
	require.Equal(t, expected, restored.Snapshot(), "恢复后的键和值不借用输入")

	record, _ := restored.Get("key")
	record.Value[0] = 'x'
	require.Equal(t, expected, restored.Snapshot())
	apply(t, &store, Command{Operation: Delete, Key: "key", ExpectedRevision: 1})
	require.Equal(t, expected, restored.Snapshot(), "恢复实例与原实例独立")
	apply(t, restored, Command{Operation: Put, Key: "key", ExpectedRevision: 1, Value: []byte("next")})
	require.Equal(t, expected, history, "历史快照不随状态变化")
	_, exists := store.Get("key")
	require.False(t, exists)
}

func TestSnapshot_Order(t *testing.T) {
	first := Store{index: 5, records: make(map[string]Record)}
	second := Store{index: 5, records: make(map[string]Record)}
	for _, key := range []string{"c", "a", "b"} {
		first.records[key] = Record{Revision: uint64(key[0] - 'a' + 1)}
		first.bytes += len(key)
	}
	for _, key := range []string{"b", "a", "c"} {
		second.records[key] = first.records[key]
		second.bytes += len(key)
	}
	require.Equal(t, first.Snapshot(), second.Snapshot(), "map 插入顺序不影响快照")
}

// snapshotData 重新计算校验和，用于验证损坏负载不能只依赖校验和拒绝。
func snapshotData(payload []byte) []byte {
	checksum := sha256.Sum256(payload)
	data := append([]byte(snapshotHeader), checksum[:]...)
	return append(data, payload...)
}

func snapshotPayload(index uint64, keys ...string) []byte {
	payload := binary.LittleEndian.AppendUint32(nil, snapshotVersion)
	payload = binary.LittleEndian.AppendUint64(payload, index)
	payload = binary.LittleEndian.AppendUint32(payload, uint32(len(keys)))
	for _, key := range keys {
		payload = binary.LittleEndian.AppendUint32(payload, uint32(len(key)))
		payload = binary.LittleEndian.AppendUint32(payload, 1)
		payload = binary.LittleEndian.AppendUint64(payload, 1)
		payload = append(payload, key...)
		payload = append(payload, 'v')
	}
	return payload
}

func TestSnapshot_Invalid(t *testing.T) {
	validPayload := snapshotPayload(1, "key")
	valid := snapshotData(validPayload)
	reject := func(t *testing.T, data []byte) {
		t.Helper()
		store, err := Restore(data)
		require.ErrorIs(t, err, ErrInvalidSnapshot)
		require.Nil(t, store, "失败时不能暴露部分恢复状态")
	}
	t.Run("truncated", func(t *testing.T) {
		for size := range len(valid) {
			reject(t, valid[:size])
		}
		for size := range len(validPayload) {
			reject(t, snapshotData(validPayload[:size]))
		}
	})
	t.Run("checksum", func(t *testing.T) {
		for offset := range len(valid) {
			data := bytes.Clone(valid)
			data[offset] ^= 1
			reject(t, data)
		}
	})
	for _, test := range []struct {
		name   string
		offset int
		value  uint32
	}{
		{"version", 0, snapshotVersion + 1},
		{"count", 12, maxEntries + 1},
		{"missing_record", 12, 2},
		{"empty_key", 16, 0},
		{"long_key", 16, maxKeyBytes + 1},
		{"key_overflow", 16, ^uint32(0)},
		{"long_value", 20, maxValueBytes + 1},
		{"value_overflow", 20, ^uint32(0)},
		{"missing_value", 20, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := bytes.Clone(validPayload)
			binary.LittleEndian.PutUint32(payload[test.offset:], test.value)
			reject(t, snapshotData(payload))
		})
	}
	for _, revision := range []uint64{0, 2, ^uint64(0)} {
		t.Run("revision_"+strconv.FormatUint(revision, 10), func(t *testing.T) {
			payload := bytes.Clone(validPayload)
			binary.LittleEndian.PutUint64(payload[24:], revision)
			reject(t, snapshotData(payload))
		})
	}
	t.Run("duplicate_key", func(t *testing.T) { reject(t, snapshotData(snapshotPayload(1, "key", "key"))) })
	t.Run("truncated_second_record", func(t *testing.T) {
		payload := snapshotPayload(2, "a", "b")
		reject(t, snapshotData(payload[:len(payload)-3]))
	})
	t.Run("invalid_utf8", func(t *testing.T) { reject(t, snapshotData(snapshotPayload(1, string([]byte{255})))) })
	t.Run("trailing_data", func(t *testing.T) { reject(t, snapshotData(append(validPayload, 0))) })
	t.Run("zero_index", func(t *testing.T) { reject(t, snapshotData(snapshotPayload(0, "key"))) })
	t.Run("oversized", func(t *testing.T) { reject(t, make([]byte, maxSnapshotBytes+1)) })
	t.Run("state_capacity", func(t *testing.T) {
		payload := binary.LittleEndian.AppendUint32(nil, snapshotVersion)
		payload = binary.LittleEndian.AppendUint64(payload, 32)
		payload = binary.LittleEndian.AppendUint32(payload, 32)
		value := make([]byte, maxValueBytes)
		for i := range 32 {
			key := strconv.Itoa(i)
			payload = binary.LittleEndian.AppendUint32(payload, uint32(len(key)))
			payload = binary.LittleEndian.AppendUint32(payload, maxValueBytes)
			payload = binary.LittleEndian.AppendUint64(payload, uint64(i+1))
			payload = append(payload, key...)
			payload = append(payload, value...)
		}
		reject(t, snapshotData(payload))
	})
}

func FuzzSnapshot_Restore(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte(snapshotHeader))
	f.Add(snapshotPayload(1, "key"))
	f.Add(snapshotData(snapshotPayload(1, "key")))
	var empty Store
	f.Add(empty.Snapshot())
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2<<20 {
			return
		}
		// 同时探索完整格式和校验和正确的任意负载。
		for _, input := range [][]byte{data, snapshotData(data)} {
			store, err := Restore(input)
			if err != nil {
				require.Nil(t, store)
				continue
			}
			canonical := store.Snapshot()
			restored, err := Restore(canonical)
			require.NoError(t, err)
			require.Equal(t, canonical, restored.Snapshot())
			require.Equal(t, store.Index(), restored.Index())
		}
	})
}

func BenchmarkSnapshot(b *testing.B) {
	var store Store
	value := make([]byte, 64)
	for i := range 1000 {
		_, err := store.Apply(uint64(i+1), Command{Operation: Put, Key: "singleton/" + strconv.Itoa(i), Value: value})
		if err != nil {
			b.Fatal(err)
		}
	}
	data := store.Snapshot()
	b.Run("create", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(data)))
		for b.Loop() {
			store.Snapshot()
		}
	})
	b.Run("restore", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(data)))
		for b.Loop() {
			if _, err := Restore(data); err != nil {
				b.Fatal(err)
			}
		}
	})
}
