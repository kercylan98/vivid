package serialization

import (
	"fmt"
	"reflect"
	"time"
)

var builtInInstanceTypes = make(map[reflect.Type]reflect.Type)
var timeType = reflect.TypeFor[time.Time]()

type Reader struct {
	buf    []byte
	pos    int
	err    error
	budget decodeBudget
}

func RegisterBuiltInInstanceType[T any](tmpl any) {
	builtInInstanceTypes[reflect.TypeFor[T]()] = reflect.TypeOf(tmpl)
}

// NewReader 创建独立的字段读取器，可选预算以最后一份为准，读取期间输入数据应保持不变。
func NewReader(data []byte, limits ...DecodeLimits) *Reader {
	return &Reader{buf: data, budget: decodeBudget{limits: decodeLimits(limits)}}
}

func GetReader(data []byte, limits ...DecodeLimits) *Reader {
	r := readerPool.Get().(*Reader)
	r.buf = data
	r.budget = decodeBudget{limits: decodeLimits(limits)}
	return r
}

func PutReader(r *Reader) {
	r.buf = nil
	r.pos = 0
	r.err = nil
	r.budget = decodeBudget{}
	readerPool.Put(r)
}

func (r *Reader) Err() error { return r.err }

func (r *Reader) check(n int) bool {
	if r.err != nil {
		return false
	}
	if n < 0 || n > len(r.buf)-r.pos {
		r.unexpectedEOF(n)
		return false
	}
	return true
}

func (r *Reader) unexpectedEOF(n int) {
	r.err = fmt.Errorf("serialization: unexpected EOF, pos: %d, n: %d, len: %d", r.pos, n, len(r.buf))
}

func (r *Reader) invalidDestination() error {
	r.err = fmt.Errorf("serialization: Read dst must be non-nil pointer")
	return r.err
}

// Read 依次读取多个目标；已知宽度的数值可直接使用泛型 ReadInteger 或 ReadFloat。
func (r *Reader) Read(values ...any) error {
	for _, value := range values {
		if r.err != nil {
			return r.err
		}
		dst := reflect.ValueOf(value)
		if dst.Kind() != reflect.Ptr || dst.IsNil() {
			return r.invalidDestination()
		}
		r.readReflect(dst.Elem())
	}
	return r.err
}

func (r *Reader) ReadBool(dst *bool) error {
	if r.err != nil {
		return r.err
	}
	if dst == nil {
		return r.invalidDestination()
	}
	value := r.readInteger[uint8]()
	if r.err == nil {
		*dst = value != 0
	}
	return r.err
}

func (r *Reader) readData() []byte {
	n := uint64(r.readInteger[uint32]())
	if r.err != nil || n > uint64(len(r.buf)-r.pos) || !r.check(int(n)) {
		if r.err == nil {
			r.err = fmt.Errorf("serialization: unexpected EOF, length: %d", n)
		}
		return nil
	}
	data := r.buf[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return data
}

func (r *Reader) ReadString(dst *string) error {
	if r.err != nil {
		return r.err
	}
	if dst == nil {
		return r.invalidDestination()
	}
	data := r.readData()
	if r.err == nil && r.ReserveAllocation(uint64(len(data))) == nil {
		*dst = string(data)
	}
	return r.err
}

func (r *Reader) ReadBytes(dst *[]byte) error {
	if r.err != nil {
		return r.err
	}
	if dst == nil {
		return r.invalidDestination()
	}
	data := r.readData()
	if r.err == nil {
		if len(data) > cap(*dst) {
			if r.ReserveAllocation(uint64(len(data))) != nil {
				return r.err
			}
			*dst = make([]byte, len(data))
		} else {
			*dst = (*dst)[:len(data)]
		}
		copy(*dst, data)
	}
	return r.err
}
