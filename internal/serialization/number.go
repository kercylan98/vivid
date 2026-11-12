package serialization

import (
	"encoding/binary"
	"math"
	"reflect"
	"unsafe"
)

// 数值宽度仅使用 unsafe.Sizeof 的编译期结果，不进行指针转换或引用内存。

// Integer 约束具有明确编码宽度的整数，避免 int 和 uint 随平台改变宽度。
type Integer interface {
	~int8 | ~uint8 | ~int16 | ~uint16 | ~int32 | ~uint32 | ~int64 | ~uint64
}

// Float 约束具有明确编码宽度的浮点数。
type Float interface {
	~float32 | ~float64
}

// WriteInteger 按类型宽度以大端序写入整数，支持自定义整数类型。
func (w *Writer) WriteInteger[T Integer](value T) *Writer {
	size := int(unsafe.Sizeof(value))
	w.ensure(size)
	if w.err != nil {
		return w
	}
	switch unsafe.Sizeof(value) {
	case 1:
		w.buf = append(w.buf, byte(value))
	case 2:
		w.buf = binary.BigEndian.AppendUint16(w.buf, uint16(value))
	case 4:
		w.buf = binary.BigEndian.AppendUint32(w.buf, uint32(value))
	case 8:
		w.buf = binary.BigEndian.AppendUint64(w.buf, uint64(value))
	}
	return w
}

// ReadInteger 按类型宽度读取大端序整数，失败时保留目标原值。
func (r *Reader) ReadInteger[T Integer](dst *T) error {
	if r.err != nil {
		return r.err
	}
	if dst == nil {
		return r.invalidDestination()
	}
	value := r.readInteger[T]()
	if r.err == nil {
		*dst = value
	}
	return r.err
}

func (r *Reader) readInteger[T Integer]() T {
	var value T
	size := int(unsafe.Sizeof(value))
	if !r.check(size) {
		return value
	}
	data := r.buf[r.pos : r.pos+size]
	r.pos += size
	switch unsafe.Sizeof(value) {
	case 1:
		return T(data[0])
	case 2:
		return T(binary.BigEndian.Uint16(data))
	case 4:
		return T(binary.BigEndian.Uint32(data))
	default:
		return T(binary.BigEndian.Uint64(data))
	}
}

// WriteFloat 按类型宽度写入 IEEE 754 浮点数，支持自定义浮点类型。
func (w *Writer) WriteFloat[T Float](value T) *Writer {
	if unsafe.Sizeof(value) == 4 {
		return w.WriteInteger(math.Float32bits(float32(value)))
	}
	return w.WriteInteger(math.Float64bits(float64(value)))
}

// ReadFloat 按类型宽度读取 IEEE 754 浮点数，失败时保留目标原值。
func (r *Reader) ReadFloat[T Float](dst *T) error {
	if r.err != nil {
		return r.err
	}
	if dst == nil {
		return r.invalidDestination()
	}
	if unsafe.Sizeof(*dst) == 4 {
		value := r.readInteger[uint32]()
		if r.err == nil {
			*dst = T(math.Float32frombits(uint32(value)))
		}
	} else {
		value := r.readInteger[uint64]()
		if r.err == nil {
			*dst = T(math.Float64frombits(value))
		}
	}
	return r.err
}

func integerWidth(kind reflect.Kind) int {
	switch kind {
	case reflect.Int8, reflect.Uint8:
		return 1
	case reflect.Int16, reflect.Uint16:
		return 2
	case reflect.Int32, reflect.Uint32:
		return 4
	default:
		return 8
	}
}
