package messages

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
	"sync"
)

var writerPool = sync.Pool{
	New: func() interface{} {
		return NewWriter()
	},
}

// WriterOption 配置写入器的选项
type WriterOption struct {
	// ByteOrder 指定字节序，用于多字节数值的编码
	// 如果为 nil，默认使用 binary.BigEndian（大端序）
	ByteOrder binary.ByteOrder

	// Buffer 预分配的缓冲区，用于减少内存分配
	// 如果为 nil，会创建一个初始容量为 256 字节的新缓冲区
	Buffer []byte

	// Reset 如果为 true，使用 Buffer 时会先将其重置为空
	// 如果为 false，新数据会追加到 Buffer 的末尾
	Reset bool
}

// Writer 提供高性能的二进制数据写入功能
//
// Writer 使用预分配的缓冲区和零拷贝技术，提供高效的写入性能。
// 所有写入操作都支持链式调用，方便连续写入多个值。
//
// 使用示例：
//
//	w := NewWriter()
//	w.WriteUint32(123).WriteString("hello")
//	data := w.Bytes()
type Writer struct {
	buf   []byte           // 数据缓冲区
	order binary.ByteOrder // 字节序
	err   error            // 写入过程中遇到的错误
	// 预分配的小缓冲区，避免频繁分配
	i16buf [2]byte // 用于 uint16/int16
	i32buf [4]byte // 用于 uint32/int32/float32
	i64buf [8]byte // 用于 uint64/int64/float64
}

// NewWriter 创建一个新的二进制写入器
//
// opts 是可选的配置项，用于自定义写入器行为
func NewWriter(opts ...WriterOption) *Writer {
	w := &Writer{
		order: binary.BigEndian, // 默认使用大端序
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.ByteOrder != nil {
			w.order = opt.ByteOrder
		}
		if opt.Buffer != nil {
			w.buf = opt.Buffer
			if opt.Reset {
				w.buf = w.buf[:0]
			}
		}
	}

	// 如果没有提供缓冲区，创建一个初始容量为 256 字节的新缓冲区
	if w.buf == nil {
		w.buf = make([]byte, 0, 256)
	}

	return w
}

// NewWriterFromPool 从池中获取一个写入器
func NewWriterFromPool(opts ...WriterOption) *Writer {
	w := writerPool.Get().(*Writer)
	if len(opts) > 0 {
		opt := opts[0]
		if opt.ByteOrder != nil {
			w.order = opt.ByteOrder
		}
	}
	return w
}

// ReleaseWriterToPool 释放一个写入器到池中
func ReleaseWriterToPool(w *Writer) {
	w.Reset()
	writerPool.Put(w)
}

// ensureCapacity 确保缓冲区有足够的容量
//
// n 是需要写入的字节数
// 如果当前容量不足，会自动扩容（至少翻倍，或增加 n 字节，取较大值）
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

// writeByte 写入一个字节
//
// 返回 Writer 自身，支持链式调用。一旦 Writer 处于错误状态则不再写入。
func (w *Writer) writeByte(v byte) *Writer {
	if w.err != nil {
		return w
	}
	w.ensureCapacity(1)
	w.buf = append(w.buf, v)
	return w
}

// WriteMessage 将消息序列化到缓冲区：先写入消息体（带 4 字节长度前缀），再写入消息名。
// 若任一步骤失败，会回滚缓冲区到调用前长度，并保证 Bytes() 不包含部分写入的数据。
func (w *Writer) WriteMessage(message any, codec Codec) (err error) {
	startLen := len(w.buf)
	messageDesc := QueryMessageDesc(message)

	if messageDesc.IsOutside() {
		data, encErr := codec.Encode(message)
		if encErr != nil {
			return encErr
		}
		w.WriteBytesWithLength(data, LengthSize4)
		if w.err != nil {
			w.buf = w.buf[:startLen]
			return w.err
		}
	} else {
		if err = SerializeRemotingMessage(codec, w, messageDesc, message); err != nil {
			w.buf = w.buf[:startLen]
			return err
		}
		if w.err != nil {
			w.buf = w.buf[:startLen]
			return w.err
		}
	}

	if err = w.WriteFrom(messageDesc.MessageName()); err != nil {
		w.buf = w.buf[:startLen]
		return err
	}
	return nil
}

// WriteBytePtr 写入一个字节指针
func (w *Writer) WriteBytePtr(v *byte) *Writer {
	return w.writeByte(*v)
}

// WriteUint8 写入一个无符号 8 位整数
func (w *Writer) WriteUint8(v uint8) *Writer {
	return w.writeByte(byte(v))
}

// WriteUint8Ptr 写入一个无符号 8 位整数指针
func (w *Writer) WriteUint8Ptr(v *uint8) *Writer {
	return w.WriteUint8(*v)
}

// WriteInt8 写入一个有符号 8 位整数
func (w *Writer) WriteInt8(v int8) *Writer {
	return w.writeByte(byte(v))
}

// WriteInt8Ptr 写入一个有符号 8 位整数指针
func (w *Writer) WriteInt8Ptr(v *int8) *Writer {
	return w.WriteInt8(*v)
}

// WriteUint16 写入一个无符号 16 位整数（2 字节）
//
// 字节序由创建 Writer 时指定的 ByteOrder 决定。错误状态下不再写入。
func (w *Writer) WriteUint16(v uint16) *Writer {
	if w.err != nil {
		return w
	}
	w.ensureCapacity(2)
	w.order.PutUint16(w.i16buf[:], v)
	w.buf = append(w.buf, w.i16buf[:]...)
	return w
}

// WriteUint16Ptr 写入一个无符号 16 位整数指针
func (w *Writer) WriteUint16Ptr(v *uint16) *Writer {
	return w.WriteUint16(*v)
}

// WriteInt16 写入一个有符号 16 位整数（2 字节）
func (w *Writer) WriteInt16(v int16) *Writer {
	return w.WriteUint16(uint16(v))
}

// WriteInt16Ptr 写入一个有符号 16 位整数指针
func (w *Writer) WriteInt16Ptr(v *int16) *Writer {
	return w.WriteInt16(*v)
}

// WriteUint32 写入一个无符号 32 位整数（4 字节）
func (w *Writer) WriteUint32(v uint32) *Writer {
	if w.err != nil {
		return w
	}
	w.ensureCapacity(4)
	w.order.PutUint32(w.i32buf[:], v)
	w.buf = append(w.buf, w.i32buf[:]...)
	return w
}

// WriteUint32Ptr 写入一个无符号 32 位整数指针
func (w *Writer) WriteUint32Ptr(v *uint32) *Writer {
	return w.WriteUint32(*v)
}

// WriteInt32 写入一个有符号 32 位整数（4 字节）
func (w *Writer) WriteInt32(v int32) *Writer {
	return w.WriteUint32(uint32(v))
}

// WriteInt32Ptr 写入一个有符号 32 位整数指针
func (w *Writer) WriteInt32Ptr(v *int32) *Writer {
	return w.WriteInt32(*v)
}

// WriteUint64 写入一个无符号 64 位整数（8 字节）
func (w *Writer) WriteUint64(v uint64) *Writer {
	if w.err != nil {
		return w
	}
	w.ensureCapacity(8)
	w.order.PutUint64(w.i64buf[:], v)
	w.buf = append(w.buf, w.i64buf[:]...)
	return w
}

// WriteUint64Ptr 写入一个无符号 64 位整数指针
func (w *Writer) WriteUint64Ptr(v *uint64) *Writer {
	return w.WriteUint64(*v)
}

// WriteInt64 写入一个有符号 64 位整数（8 字节）
func (w *Writer) WriteInt64(v int64) *Writer {
	return w.WriteUint64(uint64(v))
}

// WriteInt64Ptr 写入一个有符号 64 位整数指针
func (w *Writer) WriteInt64Ptr(v *int64) *Writer {
	return w.WriteInt64(*v)
}

// WriteBool 写入一个布尔值
//
// true 编码为 1，false 编码为 0
func (w *Writer) WriteBool(v bool) *Writer {
	if v {
		return w.writeByte(1)
	}
	return w.writeByte(0)
}

// WriteBoolPtr 写入一个布尔值指针
func (w *Writer) WriteBoolPtr(v *bool) *Writer {
	return w.WriteBool(*v)
}

// WriteFloat32 写入一个 32 位浮点数（4 字节）
//
// 使用 IEEE 754 标准进行编码
func (w *Writer) WriteFloat32(v float32) *Writer {
	return w.WriteUint32(math.Float32bits(v))
}

// WriteFloat32Ptr 写入一个 32 位浮点数指针
func (w *Writer) WriteFloat32Ptr(v *float32) *Writer {
	return w.WriteFloat32(*v)
}

// WriteFloat64 写入一个 64 位浮点数（8 字节）
//
// 使用 IEEE 754 标准进行编码
func (w *Writer) WriteFloat64(v float64) *Writer {
	return w.WriteUint64(math.Float64bits(v))
}

// WriteFloat64Ptr 写入一个 64 位浮点数指针
func (w *Writer) WriteFloat64Ptr(v *float64) *Writer {
	return w.WriteFloat64(*v)
}

// WriteVarint 写入一个变长编码的有符号整数
//
// 使用 Google Protocol Buffers 的变长整数编码格式。错误状态下不再写入。
func (w *Writer) WriteVarint(v int64) *Writer {
	if w.err != nil {
		return w
	}
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutVarint(buf[:], v)
	w.ensureCapacity(n)
	w.buf = append(w.buf, buf[:n]...)
	return w
}

// WriteUvarint 写入一个变长编码的无符号整数
//
// 使用 Google Protocol Buffers 的变长整数编码格式。错误状态下不再写入。
func (w *Writer) WriteUvarint(v uint64) *Writer {
	if w.err != nil {
		return w
	}
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	w.ensureCapacity(n)
	w.buf = append(w.buf, buf[:n]...)
	return w
}

// WriteVarintPtr 写入一个变长编码的有符号整数指针
func (w *Writer) WriteVarintPtr(v *int64) *Writer {
	return w.WriteVarint(*v)
}

// WriteUvarintPtr 写入一个变长编码的无符号整数指针
func (w *Writer) WriteUvarintPtr(v *uint64) *Writer {
	return w.WriteUvarint(*v)
}

// WriteBytes 写入字节切片
//
// v 是要写入的字节切片，会完整写入所有数据。错误状态下不再写入。
func (w *Writer) WriteBytes(v []byte) *Writer {
	if w.err != nil {
		return w
	}
	w.ensureCapacity(len(v))
	w.buf = append(w.buf, v...)
	return w
}

// WriteBytesPtr 写入一个字节切片指针
func (w *Writer) WriteBytesPtr(v *[]byte) *Writer {
	return w.WriteBytes(*v)
}

// WriteBytesWithLength 写入带长度前缀的字节切片
//
// v 是要写入的字节切片；lengthSize 为长度字段字节数（1/2/4）。错误状态下不再写入。
func (w *Writer) WriteBytesWithLength(v []byte, lengthSize int) *Writer {
	if w.err != nil {
		return w
	}
	switch lengthSize {
	case 1:
		if len(v) > 255 {
			w.err = fmt.Errorf("bytes too long for 1-byte length: %d (max 255)", len(v))
			return w
		}
		w.WriteUint8(uint8(len(v)))
	case 2:
		if len(v) > 65535 {
			w.err = fmt.Errorf("bytes too long for 2-byte length: %d (max 65535)", len(v))
			return w
		}
		w.WriteUint16(uint16(len(v)))
	case 4:
		w.WriteUint32(uint32(len(v)))
	default:
		w.err = fmt.Errorf("invalid length size: %d (must be 1, 2, or 4)", lengthSize)
		return w
	}
	return w.WriteBytes(v)
}

// WriteString 写入一个字符串（4 字节长度前缀 + UTF-8 数据）
func (w *Writer) WriteString(v string) *Writer {
	w.WriteBytesWithLength([]byte(v), LengthSize4)
	return w
}

// WriteStringPtr 写入一个字符串指针
func (w *Writer) WriteStringPtr(v *string) *Writer {
	return w.WriteString(*v)
}

// WriteShortString 写入短字符串（1 字节长度前缀，长度不超过 255）
func (w *Writer) WriteShortString(v string) *Writer {
	return w.WriteBytesWithLength([]byte(v), LengthSize1)
}

// WriteShortStringPtr 写入一个短字符串指针
func (w *Writer) WriteShortStringPtr(v *string) *Writer {
	return w.WriteShortString(*v)
}

// Write 通用写入方法，支持任意基本类型和复杂类型
//
// v 是要写入的值，支持的类型包括：
//   - 基本类型：byte, uint8, int8, uint16, int16, uint32, int32, uint64, int64
//   - 浮点类型：float32, float64
//   - 布尔类型：bool
//   - 字符串：string
//   - 字节切片：[]byte, *[]byte（nil 指针会写入长度为 0）
//   - 复杂类型：通过反射支持切片、数组和结构体
//
// 返回 Writer 自身，支持链式调用
func (w *Writer) Write(v interface{}) *Writer {
	if w.err != nil {
		return w
	}

	switch val := v.(type) {
	case *byte:
		w.writeByte(*val)
	case byte:
		w.writeByte(val)
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
		if val != nil {
			w.WriteBytesWithLength(*val, LengthSize4)
		} else {
			w.WriteUint32(0)
		}
	case []byte:
		w.WriteBytesWithLength(val, LengthSize4)
	default:
		w.err = w.writeReflect(v)
	}
	return w
}

// writeReflect 使用反射机制写入复杂类型
//
// 支持的类型：
//   - 切片和数组：先写入长度（uint32），然后递归写入每个元素
//   - 结构体：按字段顺序递归写入所有可导出字段
//   - 指针：自动解引用，nil 指针会返回错误
func (w *Writer) writeReflect(v interface{}) error {
	rv := reflect.ValueOf(v)

	// 处理指针类型，自动解引用
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return fmt.Errorf("cannot write nil pointer: %T", v)
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		length := rv.Len()
		w.WriteUint32(uint32(length))

		// 递归写入每个元素
		for i := 0; i < length; i++ {
			if err := w.writeReflect(rv.Index(i).Interface()); err != nil {
				return err
			}
		}
		return nil

	case reflect.Struct:
		// 按字段顺序写入所有可导出字段
		typ := rv.Type()
		for i := 0; i < rv.NumField(); i++ {
			field := typ.Field(i)
			// 只处理可导出的字段（PkgPath 为空表示可导出）
			if field.PkgPath == "" {
				if err := w.writeReflect(rv.Field(i).Interface()); err != nil {
					return err
				}
			}
		}
		return nil

	default:
		w.Write(v)
		return nil
	}
}

// WriteFrom 一次性写入多个值
//
// vals 是要写入的值列表，按顺序依次写入
// 如果任何写入操作失败，会立即返回错误
func (w *Writer) WriteFrom(vals ...interface{}) error {
	for _, v := range vals {
		w.Write(v)
		if w.err != nil {
			return w.err
		}
	}
	return nil
}

// Bytes 返回当前缓冲区的内容
//
// 返回的切片直接引用内部缓冲区，修改它可能会影响 Writer 的行为
// 如果需要修改返回的数据，应该先复制一份
func (w *Writer) Bytes() []byte {
	return w.buf
}

// Len 返回已写入数据的字节数
func (w *Writer) Len() int {
	return len(w.buf)
}

// Error 返回写入过程中遇到的第一个错误
//
// 一旦发生错误，后续的所有写入操作都会失败并返回相同的错误
func (w *Writer) Error() error {
	return w.err
}

// Reset 重置写入器
//
// 清空缓冲区内容，但保留底层缓冲区的容量，避免重新分配
// 重置后，错误状态被清除，可以继续使用
func (w *Writer) Reset() *Writer {
	w.buf = w.buf[:0]
	w.err = nil
	return w
}

// WriteTo 将缓冲区内容写入到 io.Writer
//
// writer 是目标写入器
// 返回写入的字节数和可能的错误
func (w *Writer) WriteTo(writer io.Writer) (int64, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := writer.Write(w.buf)
	return int64(n), err
}

// Err 返回写入过程中遇到的错误
func (w *Writer) Err() error {
	return w.err
}
