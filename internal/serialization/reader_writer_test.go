package serialization

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testIntegerEncoding[T Integer](t *testing.T, value T, data []byte) {
	t.Helper()
	w := GetWriter()
	defer PutWriter(w)
	w.WriteInteger(value)
	require.NoError(t, w.Err())
	assert.Equal(t, data, w.Bytes())
	w2 := GetWriter()
	defer PutWriter(w2)
	w2.Write(value)
	require.NoError(t, w2.Err())
	assert.Equal(t, data, w2.Bytes())
	r := GetReader(data)
	defer PutReader(r)
	var typed T
	require.NoError(t, r.ReadInteger(&typed))
	assert.Equal(t, value, typed)
	r2 := GetReader(data)
	defer PutReader(r2)
	var dynamic T
	require.NoError(t, r2.Read(&dynamic))
	assert.Equal(t, value, dynamic)
}

func TestReaderWriter_GenericInteger(t *testing.T) {
	type State int32
	type Count uint64
	testIntegerEncoding(t, int8(-128), []byte{128})
	testIntegerEncoding(t, uint8(255), []byte{255})
	testIntegerEncoding(t, int16(-2), []byte{255, 254})
	testIntegerEncoding(t, uint16(0x1234), []byte{0x12, 0x34})
	testIntegerEncoding(t, int32(-2), []byte{255, 255, 255, 254})
	testIntegerEncoding(t, uint32(0x12345678), []byte{0x12, 0x34, 0x56, 0x78})
	testIntegerEncoding(t, int64(-2), []byte{255, 255, 255, 255, 255, 255, 255, 254})
	testIntegerEncoding(t, uint64(math.MaxUint64), []byte{255, 255, 255, 255, 255, 255, 255, 255})
	testIntegerEncoding(t, State(-2), []byte{255, 255, 255, 254})
	testIntegerEncoding(t, Count(math.MaxUint64), []byte{255, 255, 255, 255, 255, 255, 255, 255})
}

func TestReaderWriter_GenericFloat(t *testing.T) {
	type Ratio float32
	type Score float64
	w := GetWriter()
	defer PutWriter(w)
	w.WriteFloat(Ratio(1.5)).WriteFloat(Score(-2.5))
	data := []byte{0x3f, 0xc0, 0, 0, 0xc0, 4, 0, 0, 0, 0, 0, 0}
	require.NoError(t, w.Err())
	assert.Equal(t, data, w.Bytes())
	r := GetReader(data)
	defer PutReader(r)
	var ratio Ratio
	var score Score
	require.NoError(t, r.ReadFloat(&ratio))
	require.NoError(t, r.ReadFloat(&score))
	assert.Equal(t, Ratio(1.5), ratio)
	assert.Equal(t, Score(-2.5), score)

	// 自定义 float32 位于结构体和 map 值时仍只占 4 字节。
	w2 := GetWriter()
	defer PutWriter(w2)
	w2.Write(struct{ Value Ratio }{1.5}, map[uint8]Ratio{1: 1.5})
	require.NoError(t, w2.Err())
	assert.Equal(t, []byte{0x3f, 0xc0, 0, 0, 0, 0, 0, 1, 1, 0x3f, 0xc0, 0, 0}, w2.Bytes())
	r2 := GetReader(w2.Bytes())
	defer PutReader(r2)
	var record struct{ Value Ratio }
	var values map[uint8]Ratio
	require.NoError(t, r2.Read(&record, &values))
	assert.Equal(t, Ratio(1.5), record.Value)
	assert.Equal(t, Ratio(1.5), values[1])
}

func TestReaderWriter_GenericError(t *testing.T) {
	r := GetReader([]byte{1})
	defer PutReader(r)
	value := int32(42)
	err := r.ReadInteger(&value)
	require.Error(t, err)
	assert.Equal(t, int32(42), value)
	ratio := float32(1.5)
	assert.Equal(t, err, r.ReadFloat(&ratio))
	assert.Equal(t, float32(1.5), ratio)
	r2 := GetReader(nil)
	defer PutReader(r2)
	assert.Error(t, r2.ReadInteger[int32](nil))
	r3 := GetReader(nil)
	defer PutReader(r3)
	assert.Error(t, r3.ReadFloat[float32](nil))
}

func TestWriter_NilValue(t *testing.T) {
	for _, value := range []any{nil, (*int32)(nil)} {
		w := GetWriter()
		w.Write(uint8(1), value, uint8(2))
		assert.Error(t, w.Err())
		assert.Equal(t, []byte{1}, w.Bytes())
		PutWriter(w)
	}
}

func TestReader_RejectTruncatedCollection(t *testing.T) {
	for _, value := range []any{new([]uint16), new([]struct{ Value uint32 })} {
		r := GetReader([]byte{255, 255, 255, 255})
		assert.Error(t, r.Read(value))
		PutReader(r)
	}
}

func TestReader_ReuseDestination(t *testing.T) {
	values := make([]uint16, 2, 8)
	before := &values[0]
	r := GetReader([]byte{0, 0, 0, 2, 0, 3, 0, 4})
	defer PutReader(r)
	require.NoError(t, r.Read(&values))
	assert.Equal(t, []uint16{3, 4}, values)
	assert.Same(t, before, &values[0])

	type Byte uint8
	named := make([]Byte, 2, 8)
	first := &named[0]
	input := []byte{0, 0, 0, 2, 3, 4}
	r2 := GetReader(input)
	defer PutReader(r2)
	require.NoError(t, r2.Read(&named))
	assert.Equal(t, []Byte{3, 4}, named)
	assert.Same(t, first, &named[0])
	input[4] = 99
	assert.Equal(t, Byte(3), named[0])
}

func TestWriter_BytesOwnership(t *testing.T) {
	w := GetWriter()
	w.WriteInteger(uint32(42))
	data := w.Bytes()
	PutWriter(w)
	for i := 0; i < 256; i++ {
		next := GetWriter()
		next.WriteInteger(uint32(i))
		PutWriter(next)
	}
	assert.Equal(t, uint32(42), binary.BigEndian.Uint32(data))
}

func TestStructFallback_CustomValue(t *testing.T) {
	w := GetWriter()
	defer PutWriter(w)
	w.Write(structFallbackMessage{value: 42}, map[uint8]structFallbackMessage{1: {value: 43}})
	require.NoError(t, w.Err())
	assert.Equal(t, []byte{0, 0, 0, 42, 0, 0, 0, 1, 1, 0, 0, 0, 43}, w.Bytes())
}

func TestStructFallback_StickyCodecError(t *testing.T) {
	type message struct {
		MessageEncoderFN
		MessageDecoderFN
	}
	value := &message{
		MessageEncoderFN: func(w *Writer, message any) error {
			w.Write(make(chan int))
			return nil
		},
		MessageDecoderFN: func(r *Reader, message any) error {
			var value uint64
			r.ReadInteger(&value)
			return nil
		},
	}
	w := GetWriter()
	defer PutWriter(w)
	w.Write(value)
	require.Error(t, w.Err())
	r := GetReader([]byte{1})
	defer PutReader(r)
	require.Error(t, r.Read(value))
}
