package serialization

import (
	"fmt"
	"math"
	"reflect"
	"time"
)

func (r *Reader) readReflect(v reflect.Value) {
	if r.err != nil {
		return
	}
	switch v.Kind() {
	case reflect.Ptr:
		value := reflect.New(v.Type().Elem())
		r.readReflect(value.Elem())
		if r.err == nil {
			v.Set(value)
		}
	case reflect.Interface:
		if typ, ok := builtInInstanceTypes[v.Type()]; ok {
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
		for i := 0; i < v.Len() && r.err == nil; i++ {
			r.readReflect(v.Index(i))
		}
	case reflect.Map:
		n := uint64(r.readInteger[uint32]())
		if r.err != nil {
			return
		}
		result := reflect.MakeMap(v.Type())
		for i := uint64(0); i < n && r.err == nil; i++ {
			key, value := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
			r.readReflect(key)
			r.readReflect(value)
			if r.err == nil {
				result.SetMapIndex(key, value)
			}
		}
		if r.err == nil {
			v.Set(result)
		}
	case reflect.Struct:
		r.readStruct(v)
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
		if r.err == nil {
			v.SetString(string(data))
		}
	default:
		r.err = fmt.Errorf("serialization: unsupported type %v", v.Type())
	}
}

func (r *Reader) readStruct(v reflect.Value) {
	if v.Type() == timeType {
		value := r.readInteger[int64]()
		if r.err == nil {
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
	for _, i := range layout.fields {
		r.readReflect(v.Field(i))
		if r.err != nil {
			return
		}
	}
}

func (r *Reader) readSlice(v reflect.Value) {
	if v.Type().Elem().Kind() == reflect.Uint8 {
		data := r.readData()
		if r.err == nil {
			v.SetBytes(append(v.Bytes()[:0], data...))
		}
		return
	}
	n := uint64(r.readInteger[uint32]())
	if r.err != nil {
		return
	}
	kind := v.Type().Elem().Kind()
	var width int
	switch kind {
	case reflect.Bool:
		width = 1
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint, reflect.Uint64:
		width = integerWidth(kind)
	case reflect.Float32:
		width = 4
	case reflect.Float64:
		width = 8
	}
	if width > 0 {
		if n > uint64(len(r.buf)-r.pos)/uint64(width) {
			r.err = fmt.Errorf("serialization: unexpected EOF, slice length: %d", n)
			return
		}
		if n > uint64(v.Cap()) {
			v.Grow(int(n) - v.Len())
		}
		v.SetLen(int(n))
		for i := 0; i < v.Len(); i++ {
			r.readReflect(v.Index(i))
		}
		return
	}
	// 变长元素逐项解码后扩容，不根据未验证的长度直接分配大块内存。
	v.SetLen(0)
	for i := uint64(0); i < n && r.err == nil; i++ {
		v.Grow(1)
		v.SetLen(v.Len() + 1)
		r.readReflect(v.Index(int(i)))
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
