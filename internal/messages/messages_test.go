package messages_test

import (
	"slices"
	"testing"

	"github.com/kercylan98/vivid/internal/messages"
	"github.com/stretchr/testify/assert"
)

func Test_Read(t *testing.T) {
	t.Run("byte", func(t *testing.T) {
		var val = byte(1)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual byte
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*byte", func(t *testing.T) {
		var val = new(byte(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual byte
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("int8", func(t *testing.T) {
		var val = new(int8(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.WriteInt8Ptr(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual int8
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("*int8", func(t *testing.T) {
		var val = new(int8(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual int8
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("uint8", func(t *testing.T) {
		var val = uint8(1)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.WriteUint8(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual uint8
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*uint8", func(t *testing.T) {
		var val = new(uint8(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual uint8
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("int16", func(t *testing.T) {
		var val = int16(1)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual int16
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*int16", func(t *testing.T) {
		var val = new(int16(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual int16
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("uint16", func(t *testing.T) {
		var val = uint16(1)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual uint16
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*uint16", func(t *testing.T) {
		var val = new(uint16(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual uint16
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("int32", func(t *testing.T) {
		var val = int32(1)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual int32
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*int32", func(t *testing.T) {
		var val = new(int32(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual int32
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("uint32", func(t *testing.T) {
		var val = uint32(1)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual uint32
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*uint32", func(t *testing.T) {
		var val = new(uint32(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual uint32
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("int64", func(t *testing.T) {
		var val = int64(1)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual int64
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*int64", func(t *testing.T) {
		var val = new(int64(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual int64
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("uint64", func(t *testing.T) {
		var val = uint64(1)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual uint64
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*uint64", func(t *testing.T) {
		var val = new(uint64(1))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual uint64
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("bool", func(t *testing.T) {
		var val = true
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual bool
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*bool", func(t *testing.T) {
		var val = new(bool(true))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual bool
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("float32", func(t *testing.T) {
		var val = float32(1.0)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual float32
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*float32", func(t *testing.T) {
		var val = new(float32(1.0))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual float32
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("float64", func(t *testing.T) {
		var val = float64(1.0)
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual float64
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*float64", func(t *testing.T) {
		var val = new(float64(1.0))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual float64
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("string", func(t *testing.T) {
		var val = "hello"
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual string
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, val, actual)
	})

	t.Run("*string", func(t *testing.T) {
		var val = new(string("hello"))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual string
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("bytes", func(t *testing.T) {
		var val = []byte("hello")
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual []byte
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, string(val), string(actual))
	})

	t.Run("*bytes", func(t *testing.T) {
		var val = new([]byte("hello"))
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual []byte
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})

	t.Run("slice", func(t *testing.T) {
		var val = []int{1, 2, 3}
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual []int
		assert.NoError(t, reader.Read(&actual))

		assert.Equal(t, len(val), len(actual))
		for i := range val {
			index := slices.Index(actual, val[i])
			assert.NotEqual(t, -1, index)
		}
	})

	t.Run("*slice", func(t *testing.T) {
		var val = new([]int{1, 2, 3})
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual []int
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)

		assert.Equal(t, len(*val), len(actual))
		for i := range *val {
			index := slices.Index(actual, (*val)[i])
			assert.NotEqual(t, -1, index)
		}
	})

	t.Run("array", func(t *testing.T) {
		var val = [3]int{1, 2, 3}
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual [3]int
		assert.NoError(t, reader.Read(&actual))

		assert.Equal(t, len(val), len(actual))
		for i := range val {
			assert.Equal(t, val[i], actual[i])
		}
	})

	t.Run("*array", func(t *testing.T) {
		var val = new([3]int{1, 2, 3})
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual [3]int
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)

		assert.Equal(t, len(*val), len(actual))
		for i := range *val {
			assert.Equal(t, (*val)[i], actual[i])
		}
	})

	t.Run("map", func(t *testing.T) {
		var val = map[string]int{"hello": 1, "world": 2}
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual map[string]int
		assert.NoError(t, reader.Read(&actual))

		assert.Equal(t, len(val), len(actual))
		for k, v := range val {
			assert.Equal(t, v, actual[k])
		}
	})

	t.Run("*map", func(t *testing.T) {
		var val = new(map[string]int{"hello": 1, "world": 2})
		var writer = messages.NewWriter()
		assert.NoError(t, writer.Write(val).Err())
		var reader = messages.NewReader(writer.Bytes())
		var actual map[string]int
		assert.NoError(t, reader.Read(&actual))
		assert.Equal(t, *val, actual)
	})
}
