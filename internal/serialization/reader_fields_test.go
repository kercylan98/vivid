package serialization

import (
	"math"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader_StructFieldLayouts(t *testing.T) {
	type Number int32
	type Flag bool
	type Text string
	type Ratio float32
	type Byte uint8
	for _, value := range []any{
		true, int8(-128), int16(-32768), int32(math.MinInt32), int64(math.MinInt64), int(-42),
		uint8(255), uint16(math.MaxUint16), uint32(math.MaxUint32), uint64(math.MaxUint64), uint(42),
		float32(-1.5), float64(2.5), "字段文本", Number(-42), Flag(true), Text("自定义类型"), Ratio(1.5),
		[]bool{true, false}, []int8{-128, 127}, []int16{-32768, 32767}, []int32{math.MinInt32, math.MaxInt32},
		[]int64{math.MinInt64, math.MaxInt64}, []int{-42, 42}, []uint8{0, 255}, []uint16{0, math.MaxUint16},
		[]uint32{0, math.MaxUint32}, []uint64{0, math.MaxUint64}, []uint{0, 42},
		[]float32{-1.5, 2.5}, []float64{-1.5, 2.5}, []Number{-42, 42}, []Flag{true, false}, []Ratio{-1.5, 2.5},
		[]Byte{0, 255}, [2]uint32{1, 2}, map[string]uint32{"a": 42},
		struct{ Value uint64 }{42}, struct{ Empty struct{} }{},
	} {
		t.Run(reflect.TypeOf(value).String(), func(t *testing.T) {
			// 前后哨兵与不同对齐的字段一起验证，防止按错误偏移或宽度覆盖相邻字段。
			typ := reflect.StructOf([]reflect.StructField{
				{Name: "Prefix", Type: reflect.TypeFor[uint8]()},
				{Name: "Value", Type: reflect.TypeOf(value)},
				{Name: "Suffix", Type: reflect.TypeFor[uint8]()},
			})
			want := reflect.New(typ)
			want.Elem().Field(0).SetUint(0xaa)
			want.Elem().Field(1).Set(reflect.ValueOf(value))
			want.Elem().Field(2).SetUint(0x55)
			w := NewWriter().Write(want.Interface())
			require.NoError(t, w.Err())
			got := reflect.New(typ)
			require.NoError(t, NewReader(w.Bytes()).Read(got.Interface()))
			assert.Equal(t, want.Elem().Interface(), got.Elem().Interface())
			// 每个截断点均应报错，不能因直接访问底层存储而越界。
			for n := 0; n < len(w.Bytes()); n++ {
				partial := reflect.New(typ)
				r := NewReader(w.Bytes()[:n])
				require.Error(t, r.Read(partial.Interface()), "length: %d", n)
				assert.Zero(t, r.budget.depth)
			}
		})
	}
}

func TestReader_StructFieldPartialUpdate(t *testing.T) {
	type message struct {
		Prefix  uint8
		private uint64
		Value   uint32
		Text    string
	}
	value := message{Prefix: 9, private: 123, Value: 99, Text: "保留"}
	r := NewReader([]byte{1, 0, 0, 0})
	require.Error(t, r.Read(&value))
	assert.Equal(t, message{Prefix: 1, private: 123, Value: 99, Text: "保留"}, value)
	r = NewReader(NewWriter().Write(uint8(2), uint32(3), "abcd").Bytes(), DecodeLimits{MaxAllocationBytes: 3})
	require.ErrorIs(t, r.Read(&value), ErrDecodeLimit)
	assert.Equal(t, message{Prefix: 2, private: 123, Value: 3, Text: "保留"}, value)
}

type readerGCField struct{ Value uint32 }

func (*readerGCField) Encode(w *Writer, message any) error {
	w.WriteInteger(message.(*readerGCField).Value)
	return w.Err()
}

func (*readerGCField) Decode(r *Reader, message any) error {
	runtime.GC()
	return r.ReadInteger(&message.(*readerGCField).Value)
}

func TestReader_StructFieldGCAndOwnership(t *testing.T) {
	type message struct {
		Name    string
		Collect readerGCField
		Nested  struct {
			Name    string
			Collect readerGCField
		}
		Data    []byte
		Pointer *uint64
		Tail    readerGCField
	}
	want := message{Name: strings.Repeat("a", 128), Collect: readerGCField{Value: 1}, Data: []byte("独立缓冲"), Tail: readerGCField{Value: 3}}
	want.Nested.Name = strings.Repeat("b", 128)
	want.Nested.Collect.Value = 2
	pointer := uint64(42)
	want.Pointer = &pointer
	data := NewWriter().Write(&want).Bytes()
	for i := 0; i < 8; i++ {
		var got message
		input := append([]byte(nil), data...)
		require.NoError(t, NewReader(input).Read(&got))
		clear(input)
		runtime.GC()
		assert.Equal(t, want, got)
	}
}

func TestReader_PrimitiveSliceFloatBits(t *testing.T) {
	values32 := []float32{math.Float32frombits(0x7fc01234), math.Float32frombits(0x80000000), float32(math.Inf(1))}
	values64 := []float64{math.Float64frombits(0x7ff8000000001234), math.Float64frombits(0x8000000000000000), math.Inf(-1)}
	w := NewWriter().Write(values32, values64)
	var got32 []float32
	var got64 []float64
	require.NoError(t, NewReader(w.Bytes()).Read(&got32, &got64))
	for i, value := range values32 {
		assert.Equal(t, math.Float32bits(value), math.Float32bits(got32[i]))
	}
	for i, value := range values64 {
		assert.Equal(t, math.Float64bits(value), math.Float64bits(got64[i]))
	}
}

func FuzzReader_StructFields(f *testing.F) {
	type nested struct {
		ID     uint32
		Text   string
		Values []int16
	}
	type message struct {
		Flag   bool
		Count  int64
		Ratio  float32
		Nested nested
		Names  map[string]string
		Next   *nested
	}
	f.Add([]byte{0, 0, 0, 0})
	f.Add(NewWriter().Write(message{Flag: true, Count: -42, Ratio: 1.5, Nested: nested{ID: 3, Text: "name", Values: []int16{-1, 2}}, Next: &nested{}}).Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		var value message
		r := NewReader(data, DecodeLimits{MaxDepth: 8, MaxElements: 32, MaxAllocationBytes: 4096})
		_ = r.Read(&value)
		assert.Zero(t, r.budget.depth)
	})
}
