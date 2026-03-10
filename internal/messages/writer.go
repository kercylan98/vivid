package messages

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
)

type Writer struct {
	buf []byte // 数据缓冲区
	err error  // 写入过程中遇到的错误
}

func NewWriter() *Writer {
	w := &Writer{}
	return w
}

func (w *Writer) ensureCapacity(n int) {
	if w.err != nil {
		return
	}
	remaining := cap(w.buf) - len(w.buf)
	if remaining < n {
		// 计算新的容量：至少翻倍，或增加 n 字节
		newCap := cap(w.buf) * 2
		if newCap < cap(w.buf)+n {
			newCap = cap(w.buf) + n
		}
		// 如果当前容量为 0，至少分配 n 字节
		if newCap < n {
			newCap = n
		}
		newBuf := make([]byte, len(w.buf), newCap)
		copy(newBuf, w.buf)
		w.buf = newBuf
	}
}

func (w *Writer) WriteByte(v byte) *Writer {
	if w.err != nil {
		return w
	}
	w.ensureCapacity(1)
	w.buf = append(w.buf, v)
	return w
}

func (w *Writer) WriteMessage(message any, codec Codec) (err error) {
	messageDesc := QueryMessageDesc(message)

	if messageDesc.IsOutside() {
		data, encErr := codec.Encode(message)
		if encErr != nil {
			return encErr
		}

		if w.WriteBytes(data); w.err != nil {
			return w.err
		}
	} else {
		if w.err = SerializeRemotingMessage(codec, w, messageDesc, message); w.err != nil {
			return w.err
		}
	}

	if w.WriteFrom(messageDesc.MessageName()); w.err != nil {
		return w.err
	}
	return nil
}

func (w *Writer) WriteBytePtr(v *byte) *Writer {
	return w.WriteByte(*v)
}

func (w *Writer) WriteUint8(v uint8) *Writer {
	return w.WriteByte(byte(v))
}

func (w *Writer) WriteUint8Ptr(v *uint8) *Writer {
	return w.WriteUint8(*v)
}

func (w *Writer) WriteInt8(v int8) *Writer {
	return w.WriteByte(byte(v))
}

func (w *Writer) WriteInt8Ptr(v *int8) *Writer {
	return w.WriteInt8(*v)
}

func (w *Writer) WriteUint16(v uint16) *Writer {
	if w.err != nil {
		return w
	}
	w.ensureCapacity(2)
	w.buf = w.buf[:len(w.buf)+2]
	binary.BigEndian.PutUint16(w.buf[len(w.buf)-2:], v)
	return w
}

func (w *Writer) WriteUint16Ptr(v *uint16) *Writer {
	return w.WriteUint16(*v)
}

func (w *Writer) WriteInt16(v int16) *Writer {
	return w.WriteUint16(uint16(v))
}

func (w *Writer) WriteInt16Ptr(v *int16) *Writer {
	return w.WriteInt16(*v)
}

func (w *Writer) WriteUint32(v uint32) *Writer {
	if w.err != nil {
		return w
	}
	w.ensureCapacity(4)
	w.buf = w.buf[:len(w.buf)+4]
	binary.BigEndian.PutUint32(w.buf[len(w.buf)-4:], v)
	return w
}

func (w *Writer) WriteUint32Ptr(v *uint32) *Writer {
	return w.WriteUint32(*v)
}

func (w *Writer) WriteInt32(v int32) *Writer {
	return w.WriteUint32(uint32(v))
}

func (w *Writer) WriteInt32Ptr(v *int32) *Writer {
	return w.WriteInt32(*v)
}

func (w *Writer) WriteUint64(v uint64) *Writer {
	if w.err != nil {
		return w
	}
	w.ensureCapacity(8)
	w.buf = w.buf[:len(w.buf)+8]
	binary.BigEndian.PutUint64(w.buf[len(w.buf)-8:], v)
	return w
}

func (w *Writer) WriteUint64Ptr(v *uint64) *Writer {
	return w.WriteUint64(*v)
}

func (w *Writer) WriteInt64(v int64) *Writer {
	return w.WriteUint64(uint64(v))
}

func (w *Writer) WriteInt64Ptr(v *int64) *Writer {
	return w.WriteInt64(*v)
}

func (w *Writer) WriteBool(v bool) *Writer {
	if v {
		return w.WriteByte(1)
	}
	return w.WriteByte(0)
}

func (w *Writer) WriteBoolPtr(v *bool) *Writer {
	return w.WriteBool(*v)
}

func (w *Writer) WriteFloat32(v float32) *Writer {
	return w.WriteUint32(math.Float32bits(v))
}

func (w *Writer) WriteFloat32Ptr(v *float32) *Writer {
	return w.WriteFloat32(*v)
}

func (w *Writer) WriteFloat64(v float64) *Writer {
	return w.WriteUint64(math.Float64bits(v))
}

func (w *Writer) WriteFloat64Ptr(v *float64) *Writer {
	return w.WriteFloat64(*v)
}

func (w *Writer) WriteBytes(v []byte) *Writer {
	if w.err != nil {
		return w
	}
	w.WriteUint32(uint32(len(v)))
	w.ensureCapacity(len(v))
	w.buf = append(w.buf, v...)
	return w
}

func (w *Writer) WriteBytesPtr(v *[]byte) *Writer {
	return w.WriteBytes(*v)
}

func (w *Writer) WriteString(v string) *Writer {
	return w.WriteBytes([]byte(v))
}

func (w *Writer) WriteStringPtr(v *string) *Writer {
	return w.WriteString(*v)
}

func (w *Writer) Write(v interface{}) *Writer {
	if w.err != nil {
		return w
	}

	switch val := v.(type) {
	case *byte:
		w.WriteByte(*val)
	case byte:
		w.WriteByte(val)
	case *int8:
		w.WriteInt8(*val)
	case int8:
		w.WriteInt8(val)
	case *int16:
		w.WriteInt16(*val)
	case int16:
		w.WriteInt16(val)
	case *uint16:
		w.WriteUint16(*val)
	case uint16:
		w.WriteUint16(val)
	case *uint32:
		w.WriteUint32(*val)
	case uint32:
		w.WriteUint32(val)
	case *int32:
		w.WriteInt32(*val)
	case int32:
		w.WriteInt32(val)
	case *uint64:
		w.WriteUint64(*val)
	case uint64:
		w.WriteUint64(val)
	case *int64:
		w.WriteInt64(*val)
	case int64:
		w.WriteInt64(val)
	case *int:
		w.WriteInt64(int64(*val))
	case int:
		w.WriteInt64(int64(val))
	case *float32:
		w.WriteFloat32(*val)
	case float32:
		w.WriteFloat32(val)
	case *float64:
		w.WriteFloat64(*val)
	case float64:
		w.WriteFloat64(val)
	case *bool:
		w.WriteBool(*val)
	case bool:
		w.WriteBool(val)
	case *string:
		w.WriteString(*val)
	case string:
		w.WriteString(val)
	case *[]byte:
		w.WriteBytes(*val)
	case []byte:
		w.WriteBytes(val)
	default:
		// 对 slice、array、map 类型进行特殊处理
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Pointer:
			// 检查是否是 slice、array、map 类型
			elem := rv.Elem()
			kind := elem.Kind()
			switch kind {
			case reflect.Slice, reflect.Array, reflect.Map:
				w.Write(elem.Interface())
			default:
				w.err = fmt.Errorf("unsupported type: %T", v)
			}
		case reflect.Slice, reflect.Array:
			length := rv.Len()
			w.WriteUint32(uint32(length))
			for i := 0; i < length; i++ {
				w.Write(rv.Index(i).Interface())
			}
		case reflect.Map:
			keys := rv.MapKeys()
			w.WriteUint32(uint32(len(keys)))
			for _, key := range keys {
				w.Write(key.Interface())
				w.Write(rv.MapIndex(key).Interface())
			}
		default:
			w.err = fmt.Errorf("unsupported type: %T", v)
		}
	}
	return w
}

func (w *Writer) WriteFrom(vals ...any) error {
	for _, v := range vals {
		if w.Write(v); w.err != nil {
			return w.err
		}
	}
	return nil
}

func (w *Writer) Bytes() []byte {
	return w.buf
}

func (w *Writer) Len() int {
	return len(w.buf)
}

func (w *Writer) Reset() *Writer {
	w.buf = w.buf[:0]
	w.err = nil
	return w
}

func (w *Writer) Err() error {
	return w.err
}
