package serialization

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"time"
)

func (r *Reader) readReflect(v reflect.Value) {
	if r.err != nil {
		return
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		r.readComposite(v)
		return
	}
	switch v.Kind() {
	case reflect.Int8:
		r.readSigned[int8](v)
	case reflect.Int16:
		r.readSigned[int16](v)
	case reflect.Int32:
		r.readSigned[int32](v)
	case reflect.Int, reflect.Int64:
		r.readSigned[int64](v)
	case reflect.Uint8:
		r.readUnsigned[uint8](v)
	case reflect.Uint16:
		r.readUnsigned[uint16](v)
	case reflect.Uint32:
		r.readUnsigned[uint32](v)
	case reflect.Uint, reflect.Uint64:
		r.readUnsigned[uint64](v)
	case reflect.Bool:
		value := r.readInteger[uint8]()
		if r.err == nil {
			v.SetBool(value != 0)
		}
	case reflect.Float32:
		value := r.readInteger[uint32]()
		if r.err == nil {
			v.SetFloat(float64(math.Float32frombits(value)))
		}
	case reflect.Float64:
		value := r.readInteger[uint64]()
		if r.err == nil {
			v.SetFloat(math.Float64frombits(value))
		}
	case reflect.String:
		data := r.readData()
		if r.err == nil && r.ReserveAllocation(uint64(len(data))) == nil {
			v.SetString(string(data))
		}
	default:
		r.err = fmt.Errorf("serialization: unsupported type %v", v.Type())
	}
}

// readComposite 仅在复合类型上管理深度，标量读取不承担 defer 的开销。
func (r *Reader) readComposite(v reflect.Value) {
	if !r.enter() {
		return
	}
	defer r.leave()
	switch v.Kind() {
	case reflect.Ptr:
		if !r.reserveAllocation(1, uint64(v.Type().Elem().Size())) {
			return
		}
		value := reflect.New(v.Type().Elem())
		r.readReflect(value.Elem())
		if r.err == nil {
			v.Set(value)
		}
	case reflect.Interface:
		if typ, ok := builtInInstanceTypes[v.Type()]; ok {
			count := uint64(2) // 值类型赋给接口时还需装箱。
			if typ.Kind() == reflect.Ptr {
				count = 1
			}
			if !r.reserveAllocation(count, uint64(typ.Size())) {
				return
			}
			value := reflect.New(typ).Elem()
			r.readReflect(value)
			if r.err == nil {
				v.Set(value)
			}
		} else {
			r.err = fmt.Errorf("serialization: cannot Read into interface{}, %s", v.Type())
		}
	case reflect.Slice:
		r.readSlice(v)
	case reflect.Array:
		n := uint64(r.readInteger[uint32]())
		if r.err != nil {
			return
		}
		if n != uint64(v.Len()) {
			r.err = fmt.Errorf("serialization: array length mismatch want %d got %d", v.Len(), n)
			return
		}
		if r.ReserveElements(n) != nil {
			return
		}
		for i := 0; i < v.Len() && r.err == nil; i++ {
			r.readReflect(v.Index(i))
		}
	case reflect.Map:
		r.readMap(v)
	case reflect.Struct:
		r.readStruct(v)
	}
}

func (r *Reader) readStruct(v reflect.Value) {
	if v.Type() == timeType {
		value := r.readInteger[int64]()
		if r.err == nil && r.ReserveAllocation(uint64(timeType.Size())) == nil {
			v.Set(reflect.ValueOf(time.Unix(0, int64(value))))
		}
		return
	}
	layout := loadStructLayout(v.Type())
	if layout.custom {
		message := v.Addr().Interface()
		err := message.(MessageCodec).Decode(r, message)
		if r.err == nil {
			r.err = err
		}
		return
	}
	r.readStructFields(v.Addr().UnsafePointer(), layout)
	runtime.KeepAlive(v)
}

func (r *Reader) readSlice(v reflect.Value) {
	if read := loadSliceReader(v.Type().Elem().Kind()); read != nil {
		read(r, v.Addr().UnsafePointer())
		runtime.KeepAlive(v)
		return
	}
	n := uint64(r.readInteger[uint32]())
	if r.err != nil || r.ReserveElements(n) != nil {
		return
	}
	if n > uint64(int(^uint(0)>>1)) {
		r.err = fmt.Errorf("%w: slice length exceeds int range", ErrDecodeLimit)
		return
	}
	if n > uint64(v.Cap()) && !r.checkAllocation(n, uint64(v.Type().Elem().Size())) {
		return
	}
	// 变长元素逐项解码后扩容，不根据未验证的长度直接分配大块内存。
	v.SetLen(0)
	for i := uint64(0); i < n && r.err == nil; i++ {
		if !r.growSlice(v, int(i)+1) {
			return
		}
		v.SetLen(v.Len() + 1)
		r.readReflect(v.Index(int(i)))
	}
}

// growSlice 在扩容前按容量增长上界预留预算，不含分配器对齐开销。
func (r *Reader) growSlice(v reflect.Value, capacity int) bool {
	if capacity <= v.Cap() {
		return true
	}
	// Go 1.27 首次按所需容量申请，后续增长按所需容量的两倍预留，不复制运行时的增长算法。
	allocation := uint64(capacity)
	if v.Cap() != 0 {
		allocation *= 2
	}
	if !r.reserveAllocation(allocation, uint64(v.Type().Elem().Size())) {
		return false
	}
	v.Grow(capacity - v.Len())
	return true
}

func (r *Reader) readMap(v reflect.Value) {
	n := uint64(r.readInteger[uint32]())
	if r.err != nil || r.ReserveElements(n) != nil {
		return
	}
	// 表头、条目与反射临时值统一计入；运行时 map 布局不属于协议保证。
	if !r.reserveAllocation(1, 128) {
		return
	}
	if n == 0 {
		v.Set(reflect.MakeMap(v.Type()))
		return
	}
	// n 来自 uint32，2*n+1 不会溢出；键值大小分开计算，避免大小相加溢出。
	if !r.reserveAllocation(n, 64) ||
		!r.reserveAllocation(2*n+1, uint64(v.Type().Key().Size())) ||
		!r.reserveAllocation(2*n+1, uint64(v.Type().Elem().Size())) {
		return
	}
	result := reflect.MakeMap(v.Type())
	key, value := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
	for i := uint64(0); i < n && r.err == nil; i++ {
		key.SetZero()
		value.SetZero()
		r.readReflect(key)
		r.readReflect(value)
		if r.err == nil {
			result.SetMapIndex(key, value)
		}
	}
	if r.err == nil {
		v.Set(result)
	}
}

func (r *Reader) readSigned[T Integer](dst reflect.Value) {
	value := r.readInteger[T]()
	if r.err == nil {
		dst.SetInt(int64(value))
	}
}

func (r *Reader) readUnsigned[T Integer](dst reflect.Value) {
	value := r.readInteger[T]()
	if r.err == nil {
		dst.SetUint(uint64(value))
	}
}
