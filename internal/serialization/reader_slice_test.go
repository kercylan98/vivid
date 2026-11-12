package serialization

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader_PrimitiveSliceReuse(t *testing.T) {
	type Number int16
	type Numbers []Number
	for _, value := range []any{
		[]int8{-128, 0, 127}, []int16{-32768, 0, 32767}, []int32{-42, 0, 42}, []int64{-42, 0, 42}, []int{-42, 0, 42},
		[]uint8{0, 1, 255}, []uint16{0, 1, 65535}, []uint32{0, 1, 42}, []uint64{0, 1, 42}, []uint{0, 1, 42},
		[]bool{true, false, true}, []float32{-1.5, 0, 2.5}, []float64{-1.5, 0, 2.5}, Numbers{-42, 0, 42},
	} {
		for _, field := range []bool{false, true} {
			t.Run(reflect.TypeOf(value).String()+map[bool]string{false: "/direct", true: "/field"}[field], func(t *testing.T) {
				typ := reflect.TypeOf(value)
				if field {
					typ = reflect.StructOf([]reflect.StructField{{Name: "Values", Type: typ}})
				}
				dst := reflect.New(typ)
				values := dst.Elem()
				if field {
					values = values.Field(0)
				}
				values.Set(reflect.MakeSlice(values.Type(), 1, 8))
				alias := values.Slice(0, 8)
				data := NewWriter().Write(value, reflect.MakeSlice(values.Type(), 0, 0).Interface()).Bytes()
				r := NewReader(data, DecodeLimits{MaxAllocationBytes: 1})
				require.NoError(t, r.Read(dst.Interface()))
				assert.Equal(t, value, values.Interface())
				assert.Equal(t, alias.Pointer(), values.Pointer())
				assert.Equal(t, value, alias.Slice(0, 3).Interface())
				assert.Zero(t, r.budget.allocationBytes)
				clear(data)
				assert.Equal(t, value, values.Interface())
				// 第二个零长度集合保留已有容量，且不清空仍由别名持有的元素。
				require.NoError(t, r.Read(dst.Interface()))
				assert.Zero(t, values.Len())
				assert.Equal(t, 8, values.Cap())
				assert.Equal(t, value, alias.Slice(0, 3).Interface())
			})
		}
	}
}

func TestReader_PrimitiveSliceFailurePreservesTarget(t *testing.T) {
	for _, field := range []bool{false, true} {
		for _, reuse := range []bool{false, true} {
			data := NewWriter().Write([]uint32{1, 2, 3}).Bytes()
			for n := 0; n < len(data); n++ {
				values := make([]uint32, 1, 1)
				if reuse {
					values = make([]uint32, 1, 8)
				}
				values[0] = 99
				original := values
				message := struct{ Values []uint32 }{values}
				var dst any = &values
				if field {
					dst = &message
				}
				r := NewReader(data[:n])
				require.Error(t, r.Read(dst), "field: %v, reuse: %v, length: %d", field, reuse, n)
				if field {
					values = message.Values
				}
				assert.Equal(t, original, values)
				assert.Equal(t, cap(original), cap(values))
				assert.Same(t, &original[0], &values[0])
				assert.Zero(t, r.budget.allocationBytes)
				assert.Zero(t, r.budget.depth)
			}
		}
	}
}

func TestReader_PrimitiveSliceAllocation(t *testing.T) {
	data := NewWriter().Write([]uint32{1, 2, 3}, []uint32{4, 5, 6, 7}).Bytes()
	for _, field := range []bool{false, true} {
		values := []uint32{99}
		message := struct{ Values []uint32 }{values}
		var dst any = &values
		if field {
			dst = &message
		}
		r := NewReader(data, DecodeLimits{MaxAllocationBytes: 12})
		require.NoError(t, r.Read(dst))
		if field {
			values = message.Values
		}
		assert.Equal(t, []uint32{1, 2, 3}, values)
		assert.Equal(t, 3, cap(values))
		assert.Equal(t, uint64(12), r.budget.allocationBytes)
		original := values
		require.ErrorIs(t, r.Read(dst), ErrDecodeLimit)
		if field {
			values = message.Values
		}
		assert.Equal(t, original, values)
		assert.Same(t, &original[0], &values[0])
		assert.Zero(t, r.budget.depth)
	}
	// 32 位平台的目标整数存储占 4 字节，协议中的每个整数仍占 8 字节。
	var native []int
	r := NewReader(NewWriter().Write([]int{-1, 2, -3}).Bytes(), DecodeLimits{MaxAllocationBytes: uint64(3 * reflect.TypeFor[int]().Size())})
	require.NoError(t, r.Read(&native))
	assert.Equal(t, []int{-1, 2, -3}, native)
	assert.Equal(t, uint64(3*reflect.TypeFor[int]().Size()), r.budget.allocationBytes)
}
