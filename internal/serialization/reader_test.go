package serialization

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReader_PutReader(t *testing.T) {
	data := []byte{1, 2, 3}
	r := GetReader(nil, data)
	require.NotNil(t, r)
	assert.Equal(t, data, r.buf)
	assert.Equal(t, 0, r.pos)
	assert.Nil(t, r.Err())

	var b byte
	_ = r.Read(&b)
	assert.Equal(t, 1, r.pos)

	PutReader(r)
	assert.Nil(t, r.buf)
	assert.Equal(t, 0, r.pos)
	assert.Nil(t, r.err)
}

func TestReader_Read_RequiresNonNilPointer(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)
	w.Write(uint32(1))

	r := GetReader(nil, w.Bytes())
	defer PutReader(r)

	var x int32
	err := r.Read(x) // pass value, not pointer
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-nil pointer")

	err = r.Read(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-nil pointer")
}

func TestReader_Read_StopsOnError(t *testing.T) {
	// data too short for uint32
	data := []byte{0, 0}
	r := GetReader(nil, data)
	defer PutReader(r)

	var u uint32
	err := r.Read(&u)
	assert.Error(t, err)
	assert.Equal(t, 0, r.pos)
}

func TestReader_Read_EOF(t *testing.T) {
	data := []byte{0, 0, 0, 1} // only 4 bytes, enough for uint32
	r := GetReader(nil, data)
	defer PutReader(r)

	var a, b uint32
	require.NoError(t, r.Read(&a))
	assert.Equal(t, uint32(1), a)
	err := r.Read(&b)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
}

func TestReader_Read_Variadic(t *testing.T) {
	codec := NewVividCodec(nil)
	w := GetWriter(codec)
	defer PutWriter(w)
	w.Write(uint32(10), uint32(20), "hello")
	require.NoError(t, w.Err())

	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())
	r := GetReader(codec, data)
	defer PutReader(r)

	var a, b uint32
	var s string
	err := r.Read(&a, &b, &s)
	require.NoError(t, err)
	assert.Equal(t, uint32(10), a)
	assert.Equal(t, uint32(20), b)
	assert.Equal(t, "hello", s)
}

func TestReader_ReadReflect_InterfaceError(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)
	w.Write(uint32(1))
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(nil, data)
	defer PutReader(r)

	var i interface{}
	err := r.Read(&i)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot Read into interface")
}

func TestReader_ReadReflect_ArrayLengthMismatch(t *testing.T) {
	// write array length 3 and three int32s; read into [2]int32 -> length mismatch
	w := GetWriter(nil)
	defer PutWriter(w)
	w.Write(uint32(3), int32(1), int32(2), int32(3))
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(nil, data)
	defer PutReader(r)

	var arr [2]int32
	err := r.Read(&arr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "array length mismatch")
}

func TestReader_Roundtrip_AllPrimitives(t *testing.T) {
	codec := NewVividCodec(nil)
	w := GetWriter(codec)
	defer PutWriter(w)

	w.Write(
		byte(1), int8(-2), uint16(3), int16(-4), uint32(5), int32(-6), uint64(7), int64(-8),
		true, false, float32(1.5), float64(2.5), []byte("raw"), "text",
	)
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
	var raw []byte
	var text string

	err := r.Read(&b, &i8, &u16, &i16, &u32, &i32, &u64, &i64, &ok, &fail, &f32, &f64, &raw, &text)
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
	assert.InDelta(t, 1.5, float64(f32), 1e-6)
	assert.Equal(t, 2.5, f64)
	assert.Equal(t, []byte("raw"), raw)
	assert.Equal(t, "text", text)
}

func TestReader_Roundtrip_PtrSliceMap(t *testing.T) {
	type S struct{ V int32 }
	codec := NewVividCodec(nil)
	w := GetWriter(codec)
	defer PutWriter(w)

	x := int32(42)
	w.Write(&x)
	w.Write([]S{{1}, {2}})
	w.Write(map[string]int32{"k": 99})
	require.NoError(t, w.Err())
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(codec, data)
	defer PutReader(r)

	var p *int32
	var sl []S
	var m map[string]int32
	err := r.Read(&p, &sl, &m)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, int32(42), *p)
	require.Len(t, sl, 2)
	assert.Equal(t, S{1}, sl[0])
	assert.Equal(t, S{2}, sl[1])
	require.Len(t, m, 1)
	assert.Equal(t, int32(99), m["k"])
}

func TestReader_Read_EmptyData(t *testing.T) {
	r := GetReader(nil, []byte{})
	defer PutReader(r)

	var u uint8
	err := r.Read(&u)
	assert.Error(t, err)
}

func TestReader_Read_NilCodecStructFallback(t *testing.T) {
	type Plain struct {
		A int32
		B string
	}
	w := GetWriter(nil)
	defer PutWriter(w)
	w.Write(Plain{A: 1, B: "x"})
	require.NoError(t, w.Err())
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(nil, data)
	defer PutReader(r)

	var p Plain
	err := r.Read(&p)
	require.NoError(t, err)
	assert.Equal(t, Plain{A: 1, B: "x"}, p)
}

func TestReader_FloatRoundtrip(t *testing.T) {
	w := GetWriter(nil)
	defer PutWriter(w)
	w.Write(float32(math.Pi), float64(-math.E))
	require.NoError(t, w.Err())
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(nil, data)
	defer PutReader(r)

	var f32 float32
	var f64 float64
	require.NoError(t, r.Read(&f32, &f64))
	assert.InDelta(t, math.Pi, float64(f32), 1e-5)
	assert.InDelta(t, -math.E, f64, 1e-10)
}
