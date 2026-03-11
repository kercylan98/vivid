package serialization

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"time"
)

type Writer struct {
	buf   []byte
	err   error
	codec *VividCodec
}

func GetWriter(codec *VividCodec) *Writer {
	w := writerPool.Get().(*Writer)
	w.codec = codec
	return w
}

func PutWriter(w *Writer) {
	w.buf = nil
	w.err = nil
	writerPool.Put(w)
}

func (w *Writer) Bytes() []byte { return w.buf }
func (w *Writer) Err() error    { return w.err }

func (w *Writer) ensure(n int) {
	if w.err != nil || cap(w.buf)-len(w.buf) >= n {
		return
	}
	need := len(w.buf) + n
	if cap(w.buf) < need {
		b := make([]byte, len(w.buf), need*2)
		copy(b, w.buf)
		w.buf = b
	}
}

func (w *Writer) Write(v ...any) *Writer {
	for _, x := range v {
		if w.err != nil {
			return w
		}
		switch val := x.(type) {
		case byte:
			w.ensure(1)
			w.buf = append(w.buf, val)
		case int8:
			w.ensure(1)
			w.buf = append(w.buf, byte(val))
		case uint16:
			w.ensure(2)
			w.buf = append(w.buf, 0, 0)
			binary.BigEndian.PutUint16(w.buf[len(w.buf)-2:], val)
		case int16:
			w.Write(uint16(val))
		case uint32:
			w.ensure(4)
			w.buf = append(w.buf, 0, 0, 0, 0)
			binary.BigEndian.PutUint32(w.buf[len(w.buf)-4:], val)
		case int32:
			w.Write(uint32(val))
		case uint64:
			w.ensure(8)
			w.buf = append(w.buf, 0, 0, 0, 0, 0, 0, 0, 0)
			binary.BigEndian.PutUint64(w.buf[len(w.buf)-8:], val)
		case int64:
			w.Write(uint64(val))
		case bool:
			if val {
				w.Write(byte(1))
			} else {
				w.Write(byte(0))
			}
		case float32:
			w.Write(math.Float32bits(val))
		case float64:
			w.Write(math.Float64bits(val))
		case []byte:
			w.Write(uint32(len(val)))
			w.ensure(len(val))
			w.buf = append(w.buf, val...)
		case string:
			w.Write([]byte(val))
		default:
			w.writeReflect(reflect.ValueOf(val))
		}
	}
	return w
}

func (w *Writer) writeReflect(v reflect.Value) {
	if w.err != nil {
		return
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if !v.IsNil() {
			w.writeReflect(v.Elem())
		}
	case reflect.Slice, reflect.Array:
		n := v.Len()
		w.Write(uint32(n))
		for i := 0; i < n; i++ {
			w.Write(v.Index(i).Interface())
		}
	case reflect.Map:
		n := v.Len()
		w.Write(uint32(n))
		for _, key := range v.MapKeys() {
			w.Write(key.Interface())
			w.Write(v.MapIndex(key).Interface())
		}
	case reflect.Struct:
		if v.Type() == timeType {
			w.Write(v.Interface().(time.Time).UnixNano())
			return
		}
		// 若能查到该类型的 MessageDesc 则用其 Encode；否则尝试 externalCodec；最后按字段递归写
		if w.codec != nil {
			if messageDesc := w.codec.FindMessageDescByType(v.Type()); messageDesc != nil {
				w.err = messageDesc.Encode(w, v.Addr().Interface())
				return
			}
		}
		if w.codec.externalCodec != nil {
			w.err = w.codec.externalCodec.Encode(w, v.Addr().Interface())
			return
		}
		for i, n := 0, v.NumField(); i < n; i++ {
			if f := v.Field(i); f.CanInterface() {
				w.Write(f.Interface())
			}
		}
	case reflect.Int8:
		w.Write(byte(v.Int()))
	case reflect.Int16:
		w.Write(uint16(v.Int()))
	case reflect.Int32:
		w.Write(uint32(v.Int()))
	case reflect.Int, reflect.Int64:
		w.Write(v.Int())
	case reflect.Uint8:
		w.Write(byte(v.Uint()))
	case reflect.Uint16:
		w.Write(uint16(v.Uint()))
	case reflect.Uint32:
		w.Write(uint32(v.Uint()))
	case reflect.Uint, reflect.Uint64:
		w.Write(v.Uint())
	case reflect.Bool:
		w.Write(v.Bool())
	case reflect.Float32, reflect.Float64:
		w.Write(v.Float())
	case reflect.String:
		w.Write(v.String())
	default:
		w.err = fmt.Errorf("serialization: unsupported type %v", v.Type())
	}
}
