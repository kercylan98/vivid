package messages

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
)

type Reader struct {
	buf []byte // 底层数据缓冲区
	pos int    // 当前读取位置
	err error  // 读取过程中遇到的错误
}

func NewReader(data []byte) *Reader {
	r := &Reader{
		buf: data,
	}
	return r
}

func (r *Reader) check(n int) bool {
	if r.err != nil {
		return false
	}
	if r.pos+n > len(r.buf) {
		r.err = io.ErrUnexpectedEOF
		return false
	}
	return true
}

func (r *Reader) ReadByte() (byte, error) {
	if !r.check(1) {
		return 0, r.err
	}
	b := r.buf[r.pos]
	r.pos++
	return b, nil
}

func (r *Reader) ReadUint8() (uint8, error) {
	return r.ReadByte()
}

func (r *Reader) ReadInt8() (int8, error) {
	b, err := r.ReadByte()
	return int8(b), err
}

func (r *Reader) ReadBool() (bool, error) {
	b, err := r.ReadByte()
	return b != 0, err
}

func (r *Reader) ReadUint16() (uint16, error) {
	if !r.check(2) {
		return 0, r.err
	}
	v := binary.BigEndian.Uint16(r.buf[r.pos:])
	r.pos += 2
	return v, nil
}

func (r *Reader) ReadInt16() (int16, error) {
	v, err := r.ReadUint16()
	return int16(v), err
}

func (r *Reader) ReadUint32() (uint32, error) {
	if !r.check(4) {
		return 0, r.err
	}
	v := binary.BigEndian.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v, nil
}

func (r *Reader) ReadInt32() (int32, error) {
	v, err := r.ReadUint32()
	return int32(v), err
}

func (r *Reader) ReadUint64() (uint64, error) {
	if !r.check(8) {
		return 0, r.err
	}
	v := binary.BigEndian.Uint64(r.buf[r.pos:])
	r.pos += 8
	return v, nil
}

func (r *Reader) ReadInt64() (int64, error) {
	v, err := r.ReadUint64()
	return int64(v), err
}

func (r *Reader) ReadFloat32() (float32, error) {
	v, err := r.ReadUint32()
	return math.Float32frombits(v), err
}

func (r *Reader) ReadFloat64() (float64, error) {
	v, err := r.ReadUint64()
	return math.Float64frombits(v), err
}

func (r *Reader) ReadBytes() ([]byte, error) {
	length, err := r.ReadUint32()
	if err != nil {
		return nil, err
	}
	if !r.check(int(length)) {
		return nil, r.err
	}
	data := r.buf[r.pos : r.pos+int(length)]
	r.pos += int(length)
	// 返回副本，避免外部修改影响内部数据
	result := make([]byte, length)
	copy(result, data)
	return result, nil
}

func (r *Reader) ReadString() (string, error) {
	data, err := r.ReadBytes()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readSliceOfType 从流中读取 length + 元素，返回新构造的 slice。
func (r *Reader) readSliceOfType(typ reflect.Type) (reflect.Value, error) {
	length, err := r.ReadUint32()
	if err != nil {
		return reflect.Value{}, err
	}
	slice := reflect.MakeSlice(typ, int(length), int(length))
	for i := 0; i < int(length); i++ {
		if err := r.Read(slice.Index(i).Addr().Interface()); err != nil {
			return reflect.Value{}, err
		}
	}
	return slice, nil
}

// readArrayOfType 从流中读取 length（并校验与类型长度一致）及元素，返回新构造的 array。
func (r *Reader) readArrayOfType(typ reflect.Type) (reflect.Value, error) {
	tmp := reflect.New(typ).Elem()
	n := tmp.Len()
	readLength, err := r.ReadUint32()
	if err != nil {
		return reflect.Value{}, err
	}
	if int(readLength) != n {
		return reflect.Value{}, fmt.Errorf("array length mismatch: expected %d, got %d", n, readLength)
	}
	for i := 0; i < n; i++ {
		if err := r.Read(tmp.Index(i).Addr().Interface()); err != nil {
			return reflect.Value{}, err
		}
	}
	return tmp, nil
}

// readMapOfType 从流中读取 length + key/value 对，返回新构造的 map。
func (r *Reader) readMapOfType(typ reflect.Type) (reflect.Value, error) {
	length, err := r.ReadUint32()
	if err != nil {
		return reflect.Value{}, err
	}
	m := reflect.MakeMap(typ)
	for i := 0; i < int(length); i++ {
		key := reflect.New(typ.Key()).Elem()
		if err := r.Read(key.Addr().Interface()); err != nil {
			return reflect.Value{}, err
		}
		value := reflect.New(typ.Elem()).Elem()
		if err := r.Read(value.Addr().Interface()); err != nil {
			return reflect.Value{}, err
		}
		m.SetMapIndex(key, value)
	}
	return m, nil
}

func (r *Reader) Read(v interface{}) error {
	if r.err != nil {
		return r.err
	}

	switch ptr := v.(type) {
	case *byte:
		// byte 和 uint8 是同一类型，统一处理
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		*ptr = b
	case *int8:
		b, err := r.ReadInt8()
		if err != nil {
			return err
		}
		*ptr = b
	case *uint16:
		val, err := r.ReadUint16()
		if err != nil {
			return err
		}
		*ptr = val
	case *int16:
		val, err := r.ReadInt16()
		if err != nil {
			return err
		}
		*ptr = val
	case *uint32:
		val, err := r.ReadUint32()
		if err != nil {
			return err
		}
		*ptr = val
	case *int32:
		val, err := r.ReadInt32()
		if err != nil {
			return err
		}
		*ptr = val
	case *uint64:
		val, err := r.ReadUint64()
		if err != nil {
			return err
		}
		*ptr = val
	case *int64:
		val, err := r.ReadInt64()
		if err != nil {
			return err
		}
		*ptr = val
	case *int:
		val, err := r.ReadInt64()
		if err != nil {
			return err
		}
		*ptr = int(val)
	case *float32:
		val, err := r.ReadFloat32()
		if err != nil {
			return err
		}
		*ptr = val
	case *float64:
		val, err := r.ReadFloat64()
		if err != nil {
			return err
		}
		*ptr = val
	case *bool:
		val, err := r.ReadBool()
		if err != nil {
			return err
		}
		*ptr = val
	case *string:
		val, err := r.ReadString()
		if err != nil {
			return err
		}
		*ptr = val
	case *[]byte:
		val, err := r.ReadBytes()
		if err != nil {
			return err
		}
		*ptr = val
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice:
			val, err := r.readSliceOfType(rv.Type())
			if err != nil {
				return err
			}
			rv.Set(val)
		case reflect.Array:
			val, err := r.readArrayOfType(rv.Type())
			if err != nil {
				return err
			}
			rv.Set(val)
		case reflect.Map:
			val, err := r.readMapOfType(rv.Type())
			if err != nil {
				return err
			}
			rv.Set(val)
		case reflect.Pointer:
			elem := rv.Elem()
			switch elem.Kind() {
			case reflect.Slice:
				val, err := r.readSliceOfType(elem.Type())
				if err != nil {
					return err
				}
				elem.Set(val)
			case reflect.Array:
				val, err := r.readArrayOfType(elem.Type())
				if err != nil {
					return err
				}
				elem.Set(val)
			case reflect.Map:
				val, err := r.readMapOfType(elem.Type())
				if err != nil {
					return err
				}
				elem.Set(val)
			case reflect.Pointer:
				innerVal := reflect.New(elem.Type().Elem()).Elem()
				if err := r.Read(innerVal.Addr().Interface()); err != nil {
					return err
				}
				elem.Set(innerVal.Addr())
			default:
				return fmt.Errorf("unsupported type: %T", v)
			}
		default:
			return fmt.Errorf("unsupported type: %T", v)
		}
	}
	return nil
}

func (r *Reader) ReadInto(vals ...interface{}) error {
	for i, v := range vals {
		if err := r.Read(v); err != nil {
			return fmt.Errorf("read index %d failed: %w", i, err)
		}
	}
	return nil
}

func (r *Reader) ReadMessage(codec Codec) (messageInstance any, err error) {
	var messageData []byte
	var messageName string
	if err = r.ReadInto(&messageData, &messageName); err != nil {
		return nil, err
	}

	if messageDesc := QueryMessageDescByName(messageName); !messageDesc.IsOutside() {
		// 内部消息反序列化
		internalReader := NewReaderFromPool(messageData)
		defer ReleaseReaderToPool(internalReader)
		messageInstance, err = DeserializeRemotingMessage(codec, internalReader, messageDesc)
		if err != nil {
			return
		}
	} else {
		// 外部消息反序列化
		messageInstance, err = codec.Decode(messageData)
		if err != nil {
			return
		}
	}
	return messageInstance, nil
}

func (r *Reader) Remaining() []byte {
	if r.err != nil {
		return nil
	}
	return r.buf[r.pos:]
}

func (r *Reader) RemainingSize() int {
	if r.err != nil {
		return 0
	}
	return len(r.buf) - r.pos
}

func (r *Reader) Pos() int {
	return r.pos
}

func (r *Reader) Seek(pos int) error {
	if pos < 0 || pos > len(r.buf) {
		return fmt.Errorf("seek position %d out of range [0, %d]", pos, len(r.buf))
	}
	r.pos = pos
	r.err = nil // 清除之前的错误
	return nil
}

func (r *Reader) Skip(n int) error {
	if !r.check(n) {
		return r.err
	}
	r.pos += n
	return nil
}

func (r *Reader) Error() error {
	return r.err
}

func (r *Reader) Reset(data []byte) {
	r.buf = data
	r.pos = 0
	r.err = nil
}
