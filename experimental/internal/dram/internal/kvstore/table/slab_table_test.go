package table_test

import (
	"testing"

	"github.com/kercylan98/vivid/experimental/internal/dram/internal/kvstore/table"
	"github.com/stretchr/testify/assert"
)

func TestSlabTable_Put(t *testing.T) {
	st := table.NewSlabTable(1024)

	t.Run("put value", func(t *testing.T) {
		err := st.Put(1, []byte("hello"))
		assert.NoError(t, err)
	})

	t.Run("put same value", func(t *testing.T) {
		err := st.Put(1, []byte("hello"))
		assert.NoError(t, err)
		err = st.Put(1, []byte("hello"))
		assert.NoError(t, err)
	})

	t.Run("put value with not enough space", func(t *testing.T) {
		var overflowValue = make([]byte, 1024)
		err := st.Put(1, overflowValue)
		assert.Equal(t, table.ErrorNotEnoughSpace, err)
	})

	t.Run("put nil value", func(t *testing.T) {
		err := st.Put(1, nil)
		assert.NoError(t, err)
	})
}

func TestSlabTable_Get(t *testing.T) {
	st := table.NewSlabTable(1024)

	t.Run("get value", func(t *testing.T) {
		err := st.Put(1, []byte("hello"))
		assert.NoError(t, err)
		value, err := st.Get(1)
		assert.NoError(t, err)
		assert.Equal(t, []byte("hello"), value)
	})

	t.Run("get value not found", func(t *testing.T) {
		value, err := st.Get(2)
		assert.Equal(t, table.ErrorKeyNotFound, err)
		assert.Nil(t, value)
	})

	t.Run("get nil value", func(t *testing.T) {
		assert.NoError(t, st.Put(3, nil))
		value, err := st.Get(3)
		assert.NoError(t, err)
		assert.Equal(t, []byte{}, value)
	})
}

func TestSlabTable_Delete(t *testing.T) {
	st := table.NewSlabTable(1024)

	t.Run("delete value", func(t *testing.T) {
		err := st.Put(1, []byte("hello"))
		assert.NoError(t, err)
		assert.True(t, st.Delete(1))
	})

	t.Run("delete value not found", func(t *testing.T) {
		assert.False(t, st.Delete(2))
	})
}

func TestSlabTable_ResetAndReuse(t *testing.T) {
	t.Run("reuse", func(t *testing.T) {
		st := table.NewSlabTable(1024)
		st.Put(1, []byte("hello"))
		st.Reset()
		assert.Equal(t, uint64(0), st.Inuse())
		assert.NotZero(t, st.ResetAt())
		st.Reuse()
		assert.Zero(t, st.ResetAt())
		assert.Zero(t, st.Invalid())
	})
}

func TestSlabTable_Iter(t *testing.T) {
	var tm = map[uint64][]byte{
		1: []byte("hello"),
		2: []byte("world"),
	}

	st := table.NewSlabTable(1024)
	for hashKey, value := range tm {
		assert.NoError(t, st.Put(hashKey, value))
	}

	var c = 0
	for hashKey, value := range st.Iter() {
		assert.Equal(t, tm[hashKey], value)
		if c++; c == len(tm)-1 {
			break
		}
	}
}

func BenchmarkSlabTable_Put(b *testing.B) {
	val := []byte("hello")
	st := table.NewSlabTable(uint64(b.N * (len(val) + table.ExposeSlabTableMetadataSize)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := st.Put(uint64(i), val); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkSlabTable_Get(b *testing.B) {
	st := table.NewSlabTable(1024)
	if err := st.Put(1, []byte("hello")); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := st.Get(1)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportAllocs()
}
