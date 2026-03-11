package serialization

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetWriter_PutWriter(t *testing.T) {
	w := GetWriter(nil)
	require.NotNil(t, w)
	assert.Nil(t, w.buf)
	assert.Nil(t, w.Err())

	w.Write(uint32(42))
	assert.NotNil(t, w.Bytes())
	assert.Len(t, w.Bytes(), 4)

	PutWriter(w)
	assert.Nil(t, w.buf)
	assert.Nil(t, w.err)
}

func TestWriter_Write_ByteAndIntegers(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write(byte(0xAB))
	assert.NoError(t, w.Err())
	assert.Equal(t, []byte{0xAB}, w.Bytes())

	w.Write(int8(-1))
	assert.Equal(t, []byte{0xAB, 0xFF}, w.Bytes())

	w.Write(uint16(0x1234))
	assert.NoError(t, w.Err())
	assert.Len(t, w.Bytes(), 4) // 1 + 1 + 2
	assert.Equal(t, byte(0x12), w.Bytes()[2])
	assert.Equal(t, byte(0x34), w.Bytes()[3])

	w.Write(int16(-2))
	w.Write(uint32(0xDEADBEEF))
	w.Write(int32(-3))
	w.Write(uint64(0x0123456789ABCDEF))
	w.Write(int64(-4))
	assert.NoError(t, w.Err())
}

func TestWriter_Write_BoolAndFloat(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write(true)
	w.Write(false)
	assert.NoError(t, w.Err())
	assert.Equal(t, []byte{1, 0}, w.Bytes())

	w2 := GetWriter(nil)
	defer PutWriter(w2)
	w2.Write(float32(1.5))
	w2.Write(float64(-2.5))
	assert.NoError(t, w2.Err())
	assert.Len(t, w2.Bytes(), 12) // 4 + 8
}

func TestWriter_Write_BytesAndString(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write([]byte("hello"))
	assert.NoError(t, w.Err())
	// 4B length + 5B data
	assert.Len(t, w.Bytes(), 9)
	assert.Equal(t, uint32(5), binary.BigEndian.Uint32(w.Bytes()[0:4]))
	assert.Equal(t, "hello", string(w.Bytes()[4:9]))

	w.Write("world")
	assert.NoError(t, w.Err())
	assert.Equal(t, uint32(5), binary.BigEndian.Uint32(w.Bytes()[9:13]))
	assert.Equal(t, "world", string(w.Bytes()[13:18]))
}

func TestWriter_Write_Variadic(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write(uint8(1), int16(2), uint32(3), "ok")
	assert.NoError(t, w.Err())
	// 1 + 2 + 4 + (4+2) for "ok"
	assert.GreaterOrEqual(t, len(w.Bytes()), 1+2+4+4+2)
}

func TestWriter_Write_StopsOnError(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write(struct{}{}) // unsupported in default writeReflect for empty struct without codec
	// empty struct goes to field iteration, no error. Try a channel which is unsupported.
	w2 := GetWriter(nil)
	defer PutWriter(w2)
	w2.Write(make(chan int))
	assert.Error(t, w2.Err())
	before := len(w2.Bytes())
	w2.Write(uint32(1))
	assert.Equal(t, before, len(w2.Bytes()), "Write after error should not append")
}

func TestWriter_WriteReflect_StructPlain(t *testing.T) {
	type S struct {
		A int32
		B string
	}
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write(S{A: 42, B: "hi"})
	assert.NoError(t, w.Err())
	// 4 bytes A + 4B len + 2B "hi"
	assert.GreaterOrEqual(t, len(w.Bytes()), 10)
}

func TestWriter_WriteReflect_SliceAndArray(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write([]int32{1, 2, 3})
	assert.NoError(t, w.Err())
	// 4B length(3) + 4*3
	assert.Equal(t, uint32(3), binary.BigEndian.Uint32(w.Bytes()[0:4]))

	w2 := GetWriter(nil)
	defer PutWriter(w2)
	arr := [2]uint16{10, 20}
	w2.Write(arr)
	assert.NoError(t, w2.Err())
	assert.Equal(t, uint32(2), binary.BigEndian.Uint32(w2.Bytes()[0:4]))
}

func TestWriter_WriteReflect_Map(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	m := map[string]int32{"a": 1, "b": 2}
	w.Write(m)
	assert.NoError(t, w.Err())
	assert.Equal(t, uint32(2), binary.BigEndian.Uint32(w.Bytes()[0:4]))
}

func TestWriter_WriteReflect_PtrAndInterface(t *testing.T) {
	x := int32(99)
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write(&x)
	assert.NoError(t, w.Err())
	assert.Equal(t, uint32(99), binary.BigEndian.Uint32(w.Bytes()[0:4]))

	var i any = int64(100)
	w2 := GetWriter(nil)
	defer PutWriter(w2)
	w2.Write(i)
	assert.NoError(t, w2.Err())
	assert.Equal(t, uint64(100), binary.BigEndian.Uint64(w2.Bytes()[0:8]))
}

func TestWriter_WriteReflect_CustomIntType(t *testing.T) {
	type Status int32
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write(Status(7))
	assert.NoError(t, w.Err())
	assert.Equal(t, uint32(7), binary.BigEndian.Uint32(w.Bytes()[0:4]))
}

func TestWriter_WriteReflect_UnsupportedType(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write(make(chan int))
	assert.Error(t, w.Err())
	assert.Contains(t, w.Err().Error(), "unsupported type")
}

func TestWriter_RoundtripWithReader_Primitives(t *testing.T) {
	codec := NewVividCodec(nil)
	w := GetWriter(codec)
	defer PutWriter(w)

	w.Write(byte(1), int8(-2), uint16(3), int16(-4), uint32(5), int32(-6), uint64(7), int64(-8),
		true, false, float32(1.5), float64(2.5), []byte("buf"), "str")

	require.NoError(t, w.Err())
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(codec, data)
	defer PutReader(r)

	var b byte
	var i8 int8
	var u16 uint16
	var i16 int16
	var u32 uint32
	var i32 int32
	var u64 uint64
	var i64 int64
	var ok, fail bool
	var f32 float32
	var f64 float64
	var buf []byte
	var s string

	err := r.Read(&b, &i8, &u16, &i16, &u32, &i32, &u64, &i64, &ok, &fail, &f32, &f64, &buf, &s)
	require.NoError(t, err)
	assert.Equal(t, byte(1), b)
	assert.Equal(t, int8(-2), i8)
	assert.Equal(t, uint16(3), u16)
	assert.Equal(t, int16(-4), i16)
	assert.Equal(t, uint32(5), u32)
	assert.Equal(t, int32(-6), i32)
	assert.Equal(t, uint64(7), u64)
	assert.Equal(t, int64(-8), i64)
	assert.True(t, ok)
	assert.False(t, fail)
	assert.Equal(t, float32(1.5), f32)
	assert.Equal(t, 2.5, f64)
	assert.Equal(t, []byte("buf"), buf)
	assert.Equal(t, "str", s)
}

func TestWriter_RoundtripWithReader_StructSliceMap(t *testing.T) {
	type S struct {
		N   int32
		Str string
	}
	codec := NewVividCodec(nil)
	w := GetWriter(codec)
	defer PutWriter(w)

	w.Write(S{N: 42, Str: "hello"})
	w.Write([]S{{1, "a"}, {2, "b"}})
	w.Write(map[string]int32{"x": 1})

	require.NoError(t, w.Err())
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(codec, data)
	defer PutReader(r)

	var s S
	var sl []S
	var m map[string]int32
	err := r.Read(&s, &sl, &m)
	require.NoError(t, err)
	assert.Equal(t, S{N: 42, Str: "hello"}, s)
	require.Len(t, sl, 2)
	assert.Equal(t, S{1, "a"}, sl[0])
	assert.Equal(t, S{2, "b"}, sl[1])
	require.Len(t, m, 1)
	assert.Equal(t, int32(1), m["x"])
}

func TestWriter_EnsureCapacity(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	for i := 0; i < 1000; i++ {
		w.Write(uint32(i))
	}
	assert.NoError(t, w.Err())
	assert.Len(t, w.Bytes(), 1000*4)
}

func TestWriter_FloatBits(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)

	w.Write(math.Float32bits(1.0))
	w.Write(math.Float64bits(-1.0))
	assert.NoError(t, w.Err())
	assert.Len(t, w.Bytes(), 12)
}
