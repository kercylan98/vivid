package clusterstore

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func apply(t *testing.T, store *Store, command Command) Result {
	t.Helper()
	result, err := store.Apply(store.Index()+1, command)
	require.NoError(t, err)
	return result
}

func TestStore_Ownership(t *testing.T) {
	var store Store
	key := "singleton/orders"
	value := []byte("node-a")
	require.Equal(t, Result{Applied: true, Revision: 1}, apply(t, &store, Command{Operation: Put, Key: key, Value: value}))
	value[0] = 'x'
	record, exists := store.Get(key)
	require.True(t, exists)
	require.Equal(t, []byte("node-a"), record.Value)
	record.Value[0] = 'y'
	record, _ = store.Get(key)
	require.Equal(t, []byte("node-a"), record.Value)

	// 两个竞争者拿到相同版本时，日志中仅首个比较成功。
	require.Equal(t, Result{Applied: true, Revision: 2}, apply(t, &store, Command{Operation: Put, Key: key, Value: []byte("node-b"), ExpectedRevision: 1}))
	require.Equal(t, Result{Revision: 2}, apply(t, &store, Command{Operation: Put, Key: key, Value: []byte("node-c"), ExpectedRevision: 1}))
	require.EqualValues(t, 3, store.Index(), "比较失败也必须消费日志位置")
	require.Equal(t, Result{Revision: 2}, apply(t, &store, Command{Operation: Delete, Key: key, ExpectedRevision: 1}))
	require.True(t, apply(t, &store, Command{Operation: Delete, Key: key, ExpectedRevision: 2}).Applied)
	require.Equal(t, Result{Applied: true, Revision: 6}, apply(t, &store, Command{Operation: Put, Key: key, Value: []byte("node-a")}))
	require.Equal(t, Result{Revision: 6}, apply(t, &store, Command{Operation: Delete, Key: key, ExpectedRevision: 1}), "旧归属不能撤销删除后重建的记录")
	require.True(t, apply(t, &store, Command{}).Applied)
	record, _ = store.Get(key)
	require.EqualValues(t, 6, record.Revision)
	require.EqualValues(t, 8, store.Index())
}

func TestStore_InvalidLog(t *testing.T) {
	var store Store
	for _, command := range []Command{
		{Operation: 255}, {Operation: Put}, {Operation: Put, Key: string([]byte{255})},
		{Operation: Put, Key: string(bytes.Repeat([]byte("x"), maxKeyBytes+1))},
		{Operation: Put, Key: "key", Value: make([]byte, maxValueBytes+1)},
		{Operation: Delete, Key: "key", Value: []byte("unexpected")}, {Key: "unexpected"},
	} {
		_, err := store.Apply(1, command)
		require.ErrorIs(t, err, ErrInvalidCommand)
		require.Zero(t, store.Index())
	}
	for _, index := range []uint64{0, 2, ^uint64(0)} {
		_, err := store.Apply(index, Command{})
		require.ErrorIs(t, err, ErrInvalidIndex)
	}
	require.True(t, apply(t, &store, Command{}).Applied)
	_, err := store.Apply(1, Command{})
	require.ErrorIs(t, err, ErrInvalidIndex)
	store.index = ^uint64(0)
	_, err = store.Apply(0, Command{})
	require.ErrorIs(t, err, ErrInvalidIndex, "不能在索引溢出后复用版本")
}

func TestStore_Capacity(t *testing.T) {
	var store Store
	value := make([]byte, maxValueBytes)
	for i := range 31 {
		key := string(rune('a' + i))
		require.True(t, apply(t, &store, Command{Operation: Put, Key: key, Value: value}).Applied)
	}
	result := apply(t, &store, Command{Operation: Put, Key: "overflow", Value: value})
	require.False(t, result.Applied)
	require.ErrorIs(t, result.Err, ErrCapacity)
	require.EqualValues(t, 32, store.Index())
	_, exists := store.Get("overflow")
	require.False(t, exists)
	// 正好达到预算时仍允许写入，拒绝新记录不能改变已有版本。
	exact := apply(t, &store, Command{Operation: Put, Key: "exact", Value: value[:maxValueBytes-31-len("exact")]})
	require.True(t, exact.Applied)
	require.Equal(t, maxStateBytes, store.bytes)
	require.ErrorIs(t, apply(t, &store, Command{Operation: Put, Key: "exact", ExpectedRevision: exact.Revision, Value: value}).Err, ErrCapacity)
	record, _ := store.Get("exact")
	require.Equal(t, exact.Revision, record.Revision)
	require.True(t, apply(t, &store, Command{Operation: Delete, Key: "exact", ExpectedRevision: exact.Revision}).Applied)
	// 缩小现有记录后能重新写入，替换不重复计算键的空间。
	require.True(t, apply(t, &store, Command{Operation: Put, Key: "a", ExpectedRevision: 1}).Applied)
	require.True(t, apply(t, &store, Command{Operation: Put, Key: "overflow", Value: value}).Applied)
	data := store.Snapshot()
	require.LessOrEqual(t, len(data), maxSnapshotBytes)
	restored, err := Restore(data)
	require.NoError(t, err)
	require.Equal(t, store.bytes, restored.bytes)
}

func TestStore_EntryLimit(t *testing.T) {
	var store Store
	for i := range maxEntries {
		require.True(t, apply(t, &store, Command{Operation: Put, Key: strconv.Itoa(i)}).Applied)
	}
	result := apply(t, &store, Command{Operation: Put, Key: "one-more"})
	require.ErrorIs(t, result.Err, ErrCapacity)
	record, _ := store.Get("0")
	require.True(t, apply(t, &store, Command{Operation: Put, Key: "0", ExpectedRevision: record.Revision, Value: []byte("replace")}).Applied)
	require.True(t, apply(t, &store, Command{Operation: Delete, Key: "0", ExpectedRevision: store.Index()}).Applied)
	require.True(t, apply(t, &store, Command{Operation: Put, Key: "one-more"}).Applied)
}

func BenchmarkStore_CompareAndSwap(b *testing.B) {
	var store Store
	var revision uint64
	command := Command{Operation: Put, Key: "singleton/orders", Value: []byte("node-a")}
	b.ReportAllocs()
	for b.Loop() {
		command.ExpectedRevision = revision
		result, err := store.Apply(store.Index()+1, command)
		if err != nil || !result.Applied {
			b.Fatal(result, err)
		}
		revision = result.Revision
	}
}

func BenchmarkStore_Conflict(b *testing.B) {
	var store Store
	command := Command{Operation: Put, Key: "singleton/orders", Value: []byte("node-a")}
	if _, err := store.Apply(1, command); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		result, err := store.Apply(store.Index()+1, command)
		if err != nil || result.Applied || result.Revision != 1 {
			b.Fatal(result, err)
		}
	}
}
