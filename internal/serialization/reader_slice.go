package serialization

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"unsafe"
)

// loadSliceReader 只为无指针的基本类型选择相同内存布局的读取器，结构体字段会缓存选择结果。
func loadSliceReader(kind reflect.Kind) func(*Reader, unsafe.Pointer) {
	switch kind {
	case reflect.Int8:
		return readIntegerSlice[int8]
	case reflect.Int16:
		return readIntegerSlice[int16]
	case reflect.Int32:
		return readIntegerSlice[int32]
	case reflect.Int64:
		return readIntegerSlice[int64]
	case reflect.Uint8:
		return readByteSlice
	case reflect.Uint16:
		return readIntegerSlice[uint16]
	case reflect.Uint32, reflect.Float32:
		return readIntegerSlice[uint32]
	case reflect.Uint64, reflect.Float64:
		return readIntegerSlice[uint64]
	case reflect.Int:
		return readNativeSlice[int]
	case reflect.Uint:
		return readNativeSlice[uint]
	case reflect.Bool:
		return readBoolSlice
	default:
		return nil
	}
}

// prepareSlice 在更新目标前验证完整负载和预算；基本类型一次申请所需长度，不保留旧数组内容。
func (r *Reader) prepareSlice[T any](dst *[]T, width int) ([]T, []byte) {
	n := uint64(r.readInteger[uint32]())
	if r.err != nil || r.ReserveElements(n) != nil {
		return nil, nil
	}
	if n > uint64(int(^uint(0)>>1)) {
		r.err = fmt.Errorf("%w: slice length exceeds int range", ErrDecodeLimit)
		return nil, nil
	}
	// n 来自 uint32，基本类型的协议宽度最多为 8；乘积不会溢出 uint64。
	size := n * uint64(width)
	if size > uint64(len(r.buf)-r.pos) {
		r.err = fmt.Errorf("serialization: unexpected EOF, slice length: %d", n)
		return nil, nil
	}
	values := *dst
	if n > uint64(cap(values)) {
		var value T
		if !r.reserveAllocation(n, uint64(unsafe.Sizeof(value))) {
			return nil, nil
		}
		values = make([]T, int(n))
	} else {
		values = values[:int(n)]
	}
	*dst = values
	data := r.buf[r.pos : r.pos+int(size)] // 完整负载已验证，不会超过剩余输入或 int 范围。
	r.pos += int(size)
	return values, data
}

// readIntegerSlice 先校验整个负载，循环内不再逐元素检查 Reader 状态；浮点数按相同宽度保留原始位。
func readIntegerSlice[T Integer](r *Reader, dst unsafe.Pointer) {
	var value T
	values, data := r.prepareSlice((*[]T)(dst), int(unsafe.Sizeof(value)))
	switch unsafe.Sizeof(value) {
	case 1:
		for i := range values {
			values[i] = T(data[i])
		}
	case 2:
		for i := range values {
			values[i] = T(binary.BigEndian.Uint16(data[i*2:]))
		}
	case 4:
		for i := range values {
			values[i] = T(binary.BigEndian.Uint32(data[i*4:]))
		}
	case 8:
		for i := range values {
			values[i] = T(binary.BigEndian.Uint64(data[i*8:]))
		}
	}
}

// readNativeSlice 的线协议始终占 8 字节，目标存储宽度由当前平台决定。
func readNativeSlice[T ~int | ~uint](r *Reader, dst unsafe.Pointer) {
	values, data := r.prepareSlice((*[]T)(dst), 8)
	for i := range values {
		values[i] = T(binary.BigEndian.Uint64(data[i*8:]))
	}
}

func readBoolSlice(r *Reader, dst unsafe.Pointer) {
	values, data := r.prepareSlice((*[]bool)(dst), 1)
	for i := range values {
		values[i] = data[i] != 0
	}
}

func readByteSlice(r *Reader, dst unsafe.Pointer) {
	r.ReadBytes((*[]byte)(dst))
}
