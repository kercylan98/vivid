package serialization

import (
	"encoding/binary"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader_DecodeLimits_DefaultAndPool(t *testing.T) {
	defaults := DefaultDecodeLimits()
	assert.Equal(t, defaults, NewReader(nil).budget.limits)
	assert.Equal(t, defaults, NewReader(nil, DecodeLimits{}).budget.limits)
	limits := defaults
	limits.MaxElements = 1
	r := GetReader(nil, limits)
	require.NoError(t, r.ReserveElements(1))
	require.ErrorIs(t, r.ReserveElements(1), ErrDecodeLimit)
	PutReader(r)
	assert.Zero(t, r.budget)
	r = GetReader(nil)
	defer PutReader(r)
	assert.Equal(t, defaults, r.budget.limits)
	require.NoError(t, r.ReserveElements(2))
}

func TestReader_DecodeLimits_Elements(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    any
		dst      func() any
		elements uint64
	}{
		{"slice", []uint16{1, 2}, func() any { return new([]uint16) }, 2},
		{"array", [2]uint16{1, 2}, func() any { return new([2]uint16) }, 2},
		{"map", map[uint16]uint16{1: 2, 3: 4}, func() any { return new(map[uint16]uint16) }, 2},
		{"nested", [][]uint16{{1, 2}, {3}}, func() any { return new([][]uint16) }, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWriter().Write(tc.value, tc.value)
			require.NoError(t, w.Err())
			r := NewReader(w.Bytes(), DecodeLimits{MaxElements: tc.elements})
			require.NoError(t, r.Read(tc.dst()))
			require.ErrorIs(t, r.Read(tc.dst()), ErrDecodeLimit)
			r = NewReader(w.Bytes(), DecodeLimits{MaxElements: tc.elements - 1})
			require.ErrorIs(t, r.Read(tc.dst()), ErrDecodeLimit)
		})
	}
	// 复用已有容量仍要计入元素数。
	w := NewWriter().Write([]uint16{1, 2})
	values := make([]uint16, 0, 8)
	r := NewReader(w.Bytes(), DecodeLimits{MaxElements: 1})
	require.ErrorIs(t, r.Read(&values), ErrDecodeLimit)
	assert.Empty(t, values)
	// 零大小元素不消耗内存，但不能绕过工作量预算。
	var empty []struct{}
	r = NewReader([]byte{255, 255, 255, 255})
	require.ErrorIs(t, r.Read(&empty), ErrDecodeLimit)
	assert.Nil(t, empty)
}

func TestReader_DecodeLimits_Allocation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		dst   func() any
		bytes uint64
	}{
		{"string", "abcd", func() any { return new(string) }, 4},
		{"bytes", []byte{1, 2, 3, 4}, func() any { return new([]byte) }, 4},
		{"slice", []uint16{1, 2}, func() any { return new([]uint16) }, 4},
		{"time", time.Unix(0, 42), func() any { return new(time.Time) }, uint64(timeType.Size())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWriter().Write(tc.value, tc.value)
			require.NoError(t, w.Err())
			r := NewReader(w.Bytes(), DecodeLimits{MaxAllocationBytes: tc.bytes})
			require.NoError(t, r.Read(tc.dst()))
			err := r.Read(tc.dst())
			require.ErrorIs(t, err, ErrDecodeLimit)
			assert.Same(t, err, r.ReserveAllocation(0))
			var unchanged uint32 = 123
			assert.Same(t, err, r.ReadInteger(&unchanged))
			assert.Equal(t, uint32(123), unchanged)
			r = NewReader(w.Bytes(), DecodeLimits{MaxAllocationBytes: tc.bytes - 1})
			require.ErrorIs(t, r.Read(tc.dst()), ErrDecodeLimit)
		})
	}
	// 通用和专用入口共享预算，不按方法调用重置。
	w := NewWriter().WriteString("ab").WriteBytes([]byte{3, 4}).WriteString("c")
	r := NewReader(w.Bytes(), DecodeLimits{MaxAllocationBytes: 4, MaxElements: 1})
	var text string
	var data []byte
	require.NoError(t, r.ReadString(&text))
	require.NoError(t, r.ReadBytes(&data))
	require.ErrorIs(t, r.ReadString(&text), ErrDecodeLimit)
	assert.Equal(t, "ab", text)
	// 已有字节容量不重复扣除分配额度，原别名仍能看到更新。
	r = NewReader(NewWriter().WriteBytes([]byte{1, 2, 3, 4}).Bytes(), DecodeLimits{MaxAllocationBytes: 1})
	data = make([]byte, 0, 4)
	alias := data[:4]
	require.NoError(t, r.ReadBytes(&data))
	assert.Equal(t, []byte{1, 2, 3, 4}, alias)
}

func TestReader_DecodeLimits_ExpansionBeforeAllocation(t *testing.T) {
	type hidden struct{ data [1 << 20]byte }
	var values []hidden
	r := NewReader([]byte{0, 0, 0, 8}, DecodeLimits{MaxAllocationBytes: 1024})
	require.ErrorIs(t, r.Read(&values), ErrDecodeLimit)
	assert.Nil(t, values)
	assert.Zero(t, cap(values))
	// 指针也必须在分配对象前检查，不能先申请再拒绝。
	var ptr *hidden
	r = NewReader(nil, DecodeLimits{MaxAllocationBytes: 1024})
	require.ErrorIs(t, r.Read(&ptr), ErrDecodeLimit)
	assert.Nil(t, ptr)
	// 默认预算同样覆盖字节数很少、对象展开很大的输入。
	r = NewReader([]byte{0, 0, 0, 100})
	require.ErrorIs(t, r.Read(&values), ErrDecodeLimit)
	assert.Nil(t, values)
}

func TestReader_DecodeLimits_SliceGrowth(t *testing.T) {
	type entry struct{ Value uint32 }
	w := NewWriter().Write([]entry{{1}, {2}, {3}})
	// 多次扩容累计扣除预算，旧数组不退还额度；使用已有容量则无需再次申请。
	var values []entry
	r := NewReader(w.Bytes())
	require.NoError(t, r.Read(&values))
	assert.Equal(t, []entry{{1}, {2}, {3}}, values)
	allocation := r.budget.allocationBytes
	require.Greater(t, allocation, uint64(len(values))*uint64(reflect.TypeFor[entry]().Size()))
	values = nil
	r = NewReader(w.Bytes(), DecodeLimits{MaxAllocationBytes: allocation - 1})
	require.ErrorIs(t, r.Read(&values), ErrDecodeLimit)
	assert.Less(t, len(values), 3)
	r = NewReader(w.Bytes(), DecodeLimits{MaxAllocationBytes: allocation})
	values = nil
	require.NoError(t, r.Read(&values))
}

type budgetRecursive struct{ Next *budgetRecursive }
type budgetReentrant struct{}

func (*budgetReentrant) Encode(*Writer, any) error           { return nil }
func (*budgetReentrant) Decode(r *Reader, message any) error { return r.Read(message) }

func TestReader_DecodeLimits_Depth(t *testing.T) {
	w := NewWriter().Write(struct{ Value []uint16 }{[]uint16{42}})
	var value struct{ Value []uint16 }
	r := NewReader(w.Bytes(), DecodeLimits{MaxDepth: 2})
	require.NoError(t, r.Read(&value))
	// 深度在退出复合类型后恢复，不累计顺序读取的层数。
	w = NewWriter().Write(value, value)
	r = NewReader(w.Bytes(), DecodeLimits{MaxDepth: 2})
	require.NoError(t, r.Read(&value, &value))
	r = NewReader(w.Bytes(), DecodeLimits{MaxDepth: 1})
	require.ErrorIs(t, r.Read(&value), ErrDecodeLimit)
	var recursive budgetRecursive
	require.ErrorIs(t, NewReader(nil).Read(&recursive), ErrDecodeLimit)
	var reentrant budgetReentrant
	r = NewReader(nil, DecodeLimits{MaxDepth: 4})
	require.ErrorIs(t, r.Read(&reentrant), ErrDecodeLimit)
	assert.Zero(t, r.budget.depth)
}

type budgetInterface interface{ budgetMarker() }
type budgetImplementation struct{ Value uint64 }

func (budgetImplementation) budgetMarker() {}

func TestReader_DecodeLimits_Interface(t *testing.T) {
	RegisterBuiltInInstanceType[budgetInterface](budgetImplementation{})
	t.Cleanup(func() { delete(builtInInstanceTypes, reflect.TypeFor[budgetInterface]()) })
	var value budgetInterface
	r := NewReader(NewWriter().WriteInteger(uint64(42)).Bytes(), DecodeLimits{MaxAllocationBytes: 15})
	require.ErrorIs(t, r.Read(&value), ErrDecodeLimit)
	assert.Nil(t, value)
	r = NewReader(NewWriter().WriteInteger(uint64(42)).Bytes(), DecodeLimits{MaxAllocationBytes: 16})
	require.NoError(t, r.Read(&value))
	assert.Equal(t, budgetImplementation{Value: 42}, value)
}

func TestReader_DecodeLimits_Map(t *testing.T) {
	w := NewWriter().Write(map[string][]byte{"a": {1}, "b": {2}})
	var value map[string][]byte
	r := NewReader(w.Bytes(), DecodeLimits{MaxAllocationBytes: 16})
	require.ErrorIs(t, r.Read(&value), ErrDecodeLimit)
	assert.Nil(t, value)
	// 复用反射临时值时必须清空，防止不同条目的字节切片共享缓冲。
	require.NoError(t, NewReader(w.Bytes()).Read(&value))
	assert.Equal(t, map[string][]byte{"a": {1}, "b": {2}}, value)
	value["a"][0] = 9
	assert.Equal(t, []byte{2}, value["b"])
}

func TestReader_DecodeLimits_Overflow(t *testing.T) {
	r := NewReader(nil, DecodeLimits{MaxAllocationBytes: math.MaxUint64, MaxElements: math.MaxUint64})
	require.NoError(t, r.ReserveAllocation(math.MaxUint64-1))
	require.ErrorIs(t, r.ReserveAllocation(2), ErrDecodeLimit)
	r = NewReader(nil, DecodeLimits{MaxElements: math.MaxUint64})
	require.NoError(t, r.ReserveElements(math.MaxUint64))
	require.ErrorIs(t, r.ReserveElements(1), ErrDecodeLimit)
	r = NewReader(nil, DecodeLimits{MaxAllocationBytes: math.MaxUint64})
	assert.False(t, r.reserveAllocation(math.MaxUint64, 2))
	require.ErrorIs(t, r.Err(), ErrDecodeLimit)
}

func TestVividCodec_DecodeLimits(t *testing.T) {
	type message struct{ hidden [1024]byte }
	c := NewVividCodec(nil)
	called := false
	require.NoError(t, c.RegisterMessageWithEncoderAndDecoder("test", "large", (*message)(nil),
		MessageEncoderFN(func(*Writer, any) error { return nil }),
		MessageDecoderFN(func(*Reader, any) error { called = true; return nil })))
	w := NewWriter().WriteString("test.large")
	r := NewReader(w.Bytes(), DecodeLimits{MaxAllocationBytes: 1024})
	_, err := c.Decode(r)
	require.ErrorIs(t, err, ErrDecodeLimit)
	assert.False(t, called)
	r = NewReader(w.Bytes(), DecodeLimits{MaxAllocationBytes: 1034})
	_, err = c.Decode(r)
	require.NoError(t, err)
	assert.True(t, called)
}

type budgetRecursiveCodec struct{ codec *VividCodec }

func (*budgetRecursiveCodec) Encode(*Writer, any) error       { return nil }
func (c *budgetRecursiveCodec) Decode(r *Reader) (any, error) { return c.codec.Decode(r) }

func TestVividCodec_DecodeLimits_Reentrant(t *testing.T) {
	external := &budgetRecursiveCodec{}
	external.codec = NewVividCodec(external)
	r := NewReader(make([]byte, 16), DecodeLimits{MaxDepth: 3})
	_, err := external.codec.Decode(r)
	require.ErrorIs(t, err, ErrDecodeLimit)
	assert.Zero(t, r.budget.depth)
}

type budgetFuzzNode struct {
	Data     []byte
	Children []budgetFuzzNode
	Labels   map[string]string
}

func FuzzReader_DecodeLimits(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0})
	f.Add([]byte{255, 255, 255, 255})
	w := NewWriter().Write(budgetFuzzNode{Data: []byte("a"), Children: []budgetFuzzNode{{}}, Labels: map[string]string{"a": "b"}})
	f.Add(w.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		limits := DecodeLimits{MaxDepth: 8, MaxElements: 32, MaxAllocationBytes: 4096}
		for _, dst := range []any{new(budgetFuzzNode), new([]struct{}), new(map[string][]byte), new(budgetRecursive)} {
			r := NewReader(data, limits)
			_ = r.Read(dst)
			if r.budget.depth != 0 || r.budget.elements > limits.MaxElements || r.budget.allocationBytes > limits.MaxAllocationBytes {
				t.Fatal("decode budget escaped", r.budget)
			}
		}
		// 任意长度前缀也需在进入集合循环前受限。
		if len(data) >= 4 && binary.BigEndian.Uint32(data[:4]) > 32 {
			var values []struct{}
			require.ErrorIs(t, NewReader(data, limits).Read(&values), ErrDecodeLimit)
		}
	})
}
