package serialization

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"time"
)

var timeType = reflect.TypeOf((*time.Time)(nil)).Elem()

type Reader struct {
	buf   []byte
	pos   int
	err   error
	codec *VividCodec
}

func GetReader(codec *VividCodec, data []byte) *Reader {
	r := readerPool.Get().(*Reader)
	r.buf = data
	r.codec = codec
	return r
}

func PutReader(r *Reader) {
	r.buf = nil
	r.pos = 0
	r.err = nil
	readerPool.Put(r)
}

func (r *Reader) Err() error { return r.err }

func (r *Reader) check(n int) bool {
	if r.err != nil {
		return false
	}
	if r.pos+n > len(r.buf) {
		r.err = fmt.Errorf("serialization: unexpected EOF")
		return false
	}
	return true
}

func (r *Reader) readByte() (byte, error) {
	if !r.check(1) {
		return 0, r.err
	}
	b := r.buf[r.pos]
	r.pos++
	return b, nil
}

func (r *Reader) readUint16() (uint16, error) {
	if !r.check(2) {
		return 0, r.err
	}
	v := binary.BigEndian.Uint16(r.buf[r.pos:])
	r.pos += 2
	return v, nil
}

func (r *Reader) readInt16() (int16, error) {
	v, err := r.readUint16()
	return int16(v), err
}

func (r *Reader) readUint32() (uint32, error) {
	if !r.check(4) {
		return 0, r.err
	}
	v := binary.BigEndian.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v, nil
}

func (r *Reader) readInt32() (int32, error) {
	v, err := r.readUint32()
	return int32(v), err
}

func (r *Reader) readUint64() (uint64, error) {
	if !r.check(8) {
		return 0, r.err
	}
	v := binary.BigEndian.Uint64(r.buf[r.pos:])
	r.pos += 8
	return v, nil
}

func (r *Reader) readInt64() (int64, error) {
	v, err := r.readUint64()
	return int64(v), err
}

func (r *Reader) readBool() (bool, error) {
	b, err := r.readByte()
	return b != 0, err
}

func (r *Reader) readFloat32() (float32, error) {
	v, err := r.readUint32()
	return math.Float32frombits(v), err
}

func (r *Reader) readFloat64() (float64, error) {
	v, err := r.readUint64()
	return math.Float64frombits(v), err
}

func (r *Reader) readBytes() ([]byte, error) {
	n, err := r.readUint32()
	if err != nil {
		return nil, err
	}
	if !r.check(int(n)) {
		return nil, r.err
	}
	out := make([]byte, n)
	copy(out, r.buf[r.pos:r.pos+int(n)])
	r.pos += int(n)
	return out, nil
}

func (r *Reader) readString() (string, error) {
	b, err := r.readBytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Read 从缓冲区按类型依次读取并写入每个 dst，dst 必须为可寻址指针。支持基础类型与反射递归（结构体、切片、map、指针）。
func (r *Reader) Read(dst ...any) error {
	if r.err != nil {
		return r.err
	}
	for _, d := range dst {
		if r.err != nil {
			return r.err
		}
		val := reflect.ValueOf(d)
		if val.Kind() != reflect.Ptr || val.IsNil() {
			r.err = fmt.Errorf("serialization: Read dst must be non-nil pointer")
			return r.err
		}
		r.readReflect(val.Elem())
	}
	return r.err
}

func (r *Reader) readReflect(v reflect.Value) {
	if r.err != nil {
		return
	}
	switch v.Kind() {
	case reflect.Ptr:
		vv := reflect.New(v.Type().Elem())
		r.readReflect(vv.Elem())
		if r.err == nil {
			v.Set(vv)
		}
	case reflect.Interface:
		// 无法从无类型信息的数据反序列化到 interface，需上层约定类型
		r.err = fmt.Errorf("serialization: cannot Read into interface{}")
	case reflect.Slice:
		n, err := r.readUint32()
		if err != nil {
			r.err = err
			return
		}
		sl := reflect.MakeSlice(v.Type(), int(n), int(n))
		for i := 0; i < int(n); i++ {
			r.readReflect(sl.Index(i))
			if r.err != nil {
				return
			}
		}
		v.Set(sl)
	case reflect.Array:
		n, err := r.readUint32()
		if err != nil {
			r.err = err
			return
		}
		if int(n) != v.Len() {
			r.err = fmt.Errorf("serialization: array length mismatch want %d got %d", v.Len(), n)
			return
		}
		for i := 0; i < v.Len(); i++ {
			r.readReflect(v.Index(i))
			if r.err != nil {
				return
			}
		}
	case reflect.Map:
		n, err := r.readUint32()
		if err != nil {
			r.err = err
			return
		}
		m := reflect.MakeMap(v.Type())
		for i := 0; i < int(n); i++ {
			k := reflect.New(v.Type().Key()).Elem()
			r.readReflect(k)
			if r.err != nil {
				return
			}
			vv := reflect.New(v.Type().Elem()).Elem()
			r.readReflect(vv)
			if r.err != nil {
				return
			}
			m.SetMapIndex(k, vv)
		}
		v.Set(m)
	case reflect.Struct:
		if v.Type() == timeType {
			unixNano, err := r.readInt64()
			if err != nil {
				r.err = err
				return
			}
			v.Set(reflect.ValueOf(time.Unix(0, unixNano)))
			return
		}

		// 兼容 time.Time
		// 若能查到该类型的 MessageDesc 则用其 Decode；否则尝试 externalCodec；最后按字段递归读
		if r.codec != nil {
			if messageDesc := r.codec.FindMessageDescByType(v.Type()); messageDesc != nil {
				r.err = messageDesc.Decode(r, v.Addr().Interface())
				return
			}
		}
		if r.codec.externalCodec != nil {
			message, err := r.codec.externalCodec.Decode(r)
			if err != nil {
				r.err = err
				return
			}
			val := reflect.ValueOf(message)
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			v.Set(val)
			return
		}
		for i, n := 0, v.NumField(); i < n; i++ {
			if f := v.Field(i); f.CanSet() {
				r.readReflect(f)
				if r.err != nil {
					return
				}
			}
		}
	case reflect.Int8:
		x, err := r.readByte()
		if err != nil {
			r.err = err
			return
		}
		v.SetInt(int64(int8(x)))
	case reflect.Int16:
		x, err := r.readInt16()
		if err != nil {
			r.err = err
			return
		}
		v.SetInt(int64(x))
	case reflect.Int32:
		x, err := r.readInt32()
		if err != nil {
			r.err = err
			return
		}
		v.SetInt(int64(x))
	case reflect.Int, reflect.Int64:
		x, err := r.readInt64()
		if err != nil {
			r.err = err
			return
		}
		v.SetInt(x)
	case reflect.Uint8:
		x, err := r.readByte()
		if err != nil {
			r.err = err
			return
		}
		v.SetUint(uint64(x))
	case reflect.Uint16:
		x, err := r.readUint16()
		if err != nil {
			r.err = err
			return
		}
		v.SetUint(uint64(x))
	case reflect.Uint32:
		x, err := r.readUint32()
		if err != nil {
			r.err = err
			return
		}
		v.SetUint(uint64(x))
	case reflect.Uint, reflect.Uint64:
		x, err := r.readUint64()
		if err != nil {
			r.err = err
			return
		}
		v.SetUint(x)
	case reflect.Bool:
		x, err := r.readBool()
		if err != nil {
			r.err = err
			return
		}
		v.SetBool(x)
	case reflect.Float32:
		x, err := r.readFloat32()
		if err != nil {
			r.err = err
			return
		}
		v.SetFloat(float64(x))
	case reflect.Float64:
		x, err := r.readFloat64()
		if err != nil {
			r.err = err
			return
		}
		v.SetFloat(x)
	case reflect.String:
		x, err := r.readString()
		if err != nil {
			r.err = err
			return
		}
		v.SetString(x)
	default:
		r.err = fmt.Errorf("serialization: unsupported type %v", v.Type())
	}
}
