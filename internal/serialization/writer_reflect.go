package serialization

import (
	"fmt"
	"reflect"
	"time"
)

func (w *Writer) writeReflect(v reflect.Value) {
	if w.err != nil {
		return
	}
	if !v.IsValid() {
		w.err = fmt.Errorf("serialization: cannot Write nil value")
		return
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			w.err = fmt.Errorf("serialization: cannot Write nil %s", v.Type())
			return
		}
		w.writeReflect(v.Elem())
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			w.WriteBytes(v.Bytes())
			return
		}
		w.writeLength(v.Len())
		for i := 0; i < v.Len() && w.err == nil; i++ {
			w.writeReflect(v.Index(i))
		}
	case reflect.Map:
		w.writeLength(v.Len())
		iter := v.MapRange()
		for iter.Next() && w.err == nil {
			w.writeReflect(iter.Key())
			w.writeReflect(iter.Value())
		}
	case reflect.Struct:
		w.writeStruct(v)
	case reflect.Int8:
		w.WriteInteger(int8(v.Int()))
	case reflect.Int16:
		w.WriteInteger(int16(v.Int()))
	case reflect.Int32:
		w.WriteInteger(int32(v.Int()))
	case reflect.Int, reflect.Int64:
		w.WriteInteger(v.Int())
	case reflect.Uint8:
		w.WriteInteger(uint8(v.Uint()))
	case reflect.Uint16:
		w.WriteInteger(uint16(v.Uint()))
	case reflect.Uint32:
		w.WriteInteger(uint32(v.Uint()))
	case reflect.Uint, reflect.Uint64:
		w.WriteInteger(v.Uint())
	case reflect.Bool:
		w.WriteBool(v.Bool())
	case reflect.Float32:
		w.WriteFloat(float32(v.Float()))
	case reflect.Float64:
		w.WriteFloat(v.Float())
	case reflect.String:
		w.WriteString(v.String())
	default:
		w.err = fmt.Errorf("serialization: unsupported type %v", v.Type())
	}
}

func (w *Writer) writeStruct(v reflect.Value) {
	if v.Type() == timeType {
		w.WriteInteger(v.Interface().(time.Time).UnixNano())
		return
	}
	layout := loadStructLayout(v.Type())
	if layout.custom {
		// 值参数、map 元素等不可寻址场景仅在编解码器需要指针时复制。
		if !v.CanAddr() {
			copy := reflect.New(v.Type())
			copy.Elem().Set(v)
			v = copy.Elem()
		}
		message := v.Addr().Interface()
		err := message.(MessageCodec).Encode(w, message)
		if w.err == nil {
			w.err = err
		}
		return
	}
	for _, i := range layout.fields {
		w.writeReflect(v.Field(i))
		if w.err != nil {
			return
		}
	}
}
