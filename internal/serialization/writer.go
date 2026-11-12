package serialization

import (
	"fmt"
	"math"
	"reflect"
)

type Writer struct {
	buf []byte
	err error
}

// NewWriter 创建独立的字段写入器。
func NewWriter() *Writer { return &Writer{} }

func GetWriter() *Writer {
	w := writerPool.Get().(*Writer)
	return w
}

func PutWriter(w *Writer) {
	// 已返回的字节可能仍被网络层持有，只复用 Writer，不复用底层缓冲区。
	w.buf = nil
	w.err = nil
	writerPool.Put(w)
}

// Bytes 返回当前编码数据；归还 Writer 后数据仍有效，后续编码不会复用该缓冲区。
func (w *Writer) Bytes() []byte { return w.buf }
func (w *Writer) Err() error    { return w.err }

func (w *Writer) ensure(n int) {
	if w.err != nil || cap(w.buf)-len(w.buf) >= n {
		return
	}
	need := len(w.buf) + n
	capacity := max(64, need*2)
	buf := make([]byte, len(w.buf), capacity)
	copy(buf, w.buf)
	w.buf = buf
}

// Write 依次写入多个值；已知宽度的数值可直接使用泛型 WriteInteger 或 WriteFloat。
func (w *Writer) Write(values ...any) *Writer {
	for _, value := range values {
		if w.err != nil {
			break
		}
		switch v := value.(type) {
		case uint8:
			w.WriteInteger(v)
		case int8:
			w.WriteInteger(v)
		case uint16:
			w.WriteInteger(v)
		case int16:
			w.WriteInteger(v)
		case uint32:
			w.WriteInteger(v)
		case int32:
			w.WriteInteger(v)
		case uint64:
			w.WriteInteger(v)
		case int64:
			w.WriteInteger(v)
		case int:
			w.WriteInteger(int64(v))
		case uint:
			w.WriteInteger(uint64(v))
		case float32:
			w.WriteFloat(v)
		case float64:
			w.WriteFloat(v)
		case bool:
			w.WriteBool(v)
		case string:
			w.WriteString(v)
		case []byte:
			w.WriteBytes(v)
		default:
			w.writeReflect(reflect.ValueOf(value))
		}
	}
	return w
}

func (w *Writer) WriteBool(value bool) *Writer {
	var b uint8
	if value {
		b = 1
	}
	return w.WriteInteger(b)
}

func (w *Writer) writeLength(n int) {
	if w.err != nil {
		return
	}
	if uint64(n) > math.MaxUint32 {
		w.err = fmt.Errorf("serialization: length %d exceeds uint32", n)
		return
	}
	w.WriteInteger(uint32(n))
}

func (w *Writer) WriteString(value string) *Writer {
	w.writeLength(len(value))
	w.ensure(len(value))
	if w.err == nil {
		w.buf = append(w.buf, value...)
	}
	return w
}

func (w *Writer) WriteBytes(value []byte) *Writer {
	w.writeLength(len(value))
	w.ensure(len(value))
	if w.err == nil {
		w.buf = append(w.buf, value...)
	}
	return w
}
