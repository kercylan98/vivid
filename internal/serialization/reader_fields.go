package serialization

import (
	"math"
	"reflect"
	"unsafe"
)

// readerField 只保存字段类型和偏移，不持有解码目标或输入缓冲。
type readerField struct {
	offset uintptr
	kind   reflect.Kind
	typ    reflect.Type
	layout *structLayout
	slice  func(*Reader, unsafe.Pointer)
}

// readStructFields 的指针来自可寻址目标，字段偏移来自其真实类型；指针仅在本次调用内使用。
func (r *Reader) readStructFields(base unsafe.Pointer, layout *structLayout) {
	for i := range layout.readerFields {
		field := &layout.readerFields[i]
		dst := unsafe.Add(base, field.offset)
		switch field.kind {
		case reflect.Int8:
			r.readFieldInteger[int8](dst)
		case reflect.Int16:
			r.readFieldInteger[int16](dst)
		case reflect.Int32:
			r.readFieldInteger[int32](dst)
		case reflect.Int64:
			r.readFieldInteger[int64](dst)
		case reflect.Uint8:
			r.readFieldInteger[uint8](dst)
		case reflect.Uint16:
			r.readFieldInteger[uint16](dst)
		case reflect.Uint32:
			r.readFieldInteger[uint32](dst)
		case reflect.Uint64:
			r.readFieldInteger[uint64](dst)
		case reflect.Int:
			value := r.readInteger[int64]()
			if r.err == nil {
				*(*int)(dst) = int(value)
			}
		case reflect.Uint:
			value := r.readInteger[uint64]()
			if r.err == nil {
				*(*uint)(dst) = uint(value)
			}
		case reflect.Bool:
			value := r.readInteger[uint8]()
			if r.err == nil {
				*(*bool)(dst) = value != 0
			}
		case reflect.Float32:
			value := r.readInteger[uint32]()
			if r.err == nil {
				*(*float32)(dst) = math.Float32frombits(value)
			}
		case reflect.Float64:
			value := r.readInteger[uint64]()
			if r.err == nil {
				*(*float64)(dst) = math.Float64frombits(value)
			}
		case reflect.String:
			r.ReadString((*string)(dst))
		case reflect.Struct:
			if field.layout != nil && !field.layout.custom {
				r.readNestedStruct(dst, field.layout)
			} else {
				r.readComposite(reflect.NewAt(field.typ, dst).Elem())
			}
		case reflect.Slice:
			if field.slice != nil {
				r.readSliceField(dst, field.slice)
			} else {
				r.readComposite(reflect.NewAt(field.typ, dst).Elem())
			}
		case reflect.Array, reflect.Map, reflect.Ptr, reflect.Interface:
			r.readComposite(reflect.NewAt(field.typ, dst).Elem())
		default:
			r.readReflect(reflect.NewAt(field.typ, dst).Elem())
		}
		if r.err != nil {
			return
		}
	}
}

func (r *Reader) readFieldInteger[T Integer](dst unsafe.Pointer) {
	value := r.readInteger[T]()
	if r.err == nil {
		*(*T)(dst) = value
	}
}

func (r *Reader) readNestedStruct(dst unsafe.Pointer, layout *structLayout) {
	if !r.enter() {
		return
	}
	defer r.leave()
	r.readStructFields(dst, layout)
}

func (r *Reader) readSliceField(dst unsafe.Pointer, read func(*Reader, unsafe.Pointer)) {
	if !r.enter() {
		return
	}
	defer r.leave()
	read(r, dst)
}
