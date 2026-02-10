package messages

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sync"
)

var ErrOverflow = errors.New("overflow")

// 长度前缀字节数（用于 ReadBytesWithLength / WriteBytesWithLength）
const (
	LengthSize1 = 1 // 最大 255
	LengthSize2 = 2 // 最大 65535
	LengthSize4 = 4
)

var readerPool = sync.Pool{
	New: func() interface{} {
		return NewReader(nil)
	},
}

// ReaderOption 配置读取器的选项
type ReaderOption struct {
	// ByteOrder 指定字节序，用于多字节数值的解析
	// 如果为 nil，默认使用 binary.BigEndian（大端序）
	ByteOrder binary.ByteOrder

	// Mutable 如果为 true，会创建底层数据的可修改副本
	// 默认情况下，Reader 直接引用传入的数据，不会复制
	Mutable bool
}

// Reader 提供高性能的二进制数据读取功能
//
// Reader 使用零拷贝技术直接操作底层字节数组，提供高效的读取性能。
// 所有读取操作都会自动进行边界检查，确保不会越界访问。
//
// 使用示例：
//
//	r := NewReader(data)
//	value, err := r.ReadUint32()
//	str, err := r.ReadString()
type Reader struct {
	buf   []byte           // 底层数据缓冲区
	pos   int              // 当前读取位置
	order binary.ByteOrder // 字节序
	err   error            // 读取过程中遇到的错误
}

// NewReaderFromPool 从池中获取一个读取器
func NewReaderFromPool(data []byte, opts ...ReaderOption) *Reader {
	r := readerPool.Get().(*Reader)
	r.buf = data
	if len(opts) > 0 {
		opt := opts[0]
		if opt.ByteOrder != nil {
			r.order = opt.ByteOrder
		}
		if opt.Mutable {
			r.buf = make([]byte, len(data))
			copy(r.buf, data)
		}
	}
	return r
}

// ReleaseReaderToPool 释放一个读取器到池中
func ReleaseReaderToPool(r *Reader) {
	r.Reset(nil)
	readerPool.Put(r)
}

// NewReader 创建一个新的二进制读取器
//
// data 是要读取的字节数组，Reader 会直接引用该数组（除非设置了 Mutable 选项）
// opts 是可选的配置项，用于自定义读取器行为
func NewReader(data []byte, opts ...ReaderOption) *Reader {
	r := &Reader{
		buf:   data,
		order: binary.BigEndian, // 默认使用大端序
	}

	if len(opts) > 0 {
		opt := opts[0]
		if opt.ByteOrder != nil {
			r.order = opt.ByteOrder
		}
		// 如果需要可修改的副本，创建新的缓冲区并复制数据
		if opt.Mutable && len(data) > 0 {
			r.buf = make([]byte, len(data))
			copy(r.buf, data)
		}
	}

	return r
}

// check 检查是否有足够的字节可供读取
//
// n 是需要读取的字节数
// 返回 true 表示可以安全读取，false 表示已发生错误或数据不足
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

// ReadByte 读取一个字节
func (r *Reader) ReadByte() (byte, error) {
	if !r.check(1) {
		return 0, r.err
	}
	b := r.buf[r.pos]
	r.pos++
	return b, nil
}

// ReadUint8 读取一个无符号 8 位整数
func (r *Reader) ReadUint8() (uint8, error) {
	return r.ReadByte()
}

// ReadInt8 读取一个有符号 8 位整数
func (r *Reader) ReadInt8() (int8, error) {
	b, err := r.ReadByte()
	return int8(b), err
}

// ReadBool 读取一个布尔值
//
// 布尔值以单个字节存储：0 表示 false，非 0 表示 true
func (r *Reader) ReadBool() (bool, error) {
	b, err := r.ReadByte()
	return b != 0, err
}

// ReadUint16 读取一个无符号 16 位整数（2 字节）
//
// 字节序由创建 Reader 时指定的 ByteOrder 决定
func (r *Reader) ReadUint16() (uint16, error) {
	if !r.check(2) {
		return 0, r.err
	}
	v := r.order.Uint16(r.buf[r.pos:])
	r.pos += 2
	return v, nil
}

// ReadInt16 读取一个有符号 16 位整数（2 字节）
func (r *Reader) ReadInt16() (int16, error) {
	v, err := r.ReadUint16()
	return int16(v), err
}

// ReadUint32 读取一个无符号 32 位整数（4 字节）
func (r *Reader) ReadUint32() (uint32, error) {
	if !r.check(4) {
		return 0, r.err
	}
	v := r.order.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v, nil
}

// ReadInt32 读取一个有符号 32 位整数（4 字节）
func (r *Reader) ReadInt32() (int32, error) {
	v, err := r.ReadUint32()
	return int32(v), err
}

// ReadUint64 读取一个无符号 64 位整数（8 字节）
func (r *Reader) ReadUint64() (uint64, error) {
	if !r.check(8) {
		return 0, r.err
	}
	v := r.order.Uint64(r.buf[r.pos:])
	r.pos += 8
	return v, nil
}

// ReadInt64 读取一个有符号 64 位整数（8 字节）
func (r *Reader) ReadInt64() (int64, error) {
	v, err := r.ReadUint64()
	return int64(v), err
}

// ReadFloat32 读取一个 32 位浮点数（4 字节）
//
// 使用 IEEE 754 标准进行编码/解码
func (r *Reader) ReadFloat32() (float32, error) {
	v, err := r.ReadUint32()
	return math.Float32frombits(v), err
}

// ReadFloat64 读取一个 64 位浮点数（8 字节）
//
// 使用 IEEE 754 标准进行编码/解码
func (r *Reader) ReadFloat64() (float64, error) {
	v, err := r.ReadUint64()
	return math.Float64frombits(v), err
}

// ReadVarint 读取一个变长编码的有符号整数
//
// 使用 Google Protocol Buffers 的变长整数编码格式
// 小数值占用更少的字节，大数值占用更多字节
func (r *Reader) ReadVarint() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	v, n := binary.Varint(r.buf[r.pos:])
	if n <= 0 {
		if n == 0 {
			r.err = io.ErrUnexpectedEOF
		} else {
			r.err = ErrOverflow
		}
		return 0, r.err
	}
	r.pos += n
	return v, nil
}

// ReadUvarint 读取一个变长编码的无符号整数
//
// 使用 Google Protocol Buffers 的变长整数编码格式
func (r *Reader) ReadUvarint() (uint64, error) {
	if r.err != nil {
		return 0, r.err
	}
	v, n := binary.Uvarint(r.buf[r.pos:])
	if n <= 0 {
		if n == 0 {
			r.err = io.ErrUnexpectedEOF
		} else {
			r.err = ErrOverflow
		}
		return 0, r.err
	}
	r.pos += n
	return v, nil
}

// ReadBytes 读取指定长度的字节切片
//
// n 是要读取的字节数
// 返回的字节切片是原始数据的副本，修改它不会影响 Reader 的内部状态
func (r *Reader) ReadBytes(n int) ([]byte, error) {
	if !r.check(n) {
		return nil, r.err
	}
	data := r.buf[r.pos : r.pos+n]
	r.pos += n
	// 返回副本，避免外部修改影响内部数据
	result := make([]byte, n)
	copy(result, data)
	return result, nil
}

// ReadBytesWithLength 读取带长度前缀的字节切片
//
// lengthSize 指定长度字段的字节数，支持 1、2 或 4 字节
// 格式：先读取 lengthSize 字节的长度值，然后读取对应长度的数据
func (r *Reader) ReadBytesWithLength(lengthSize int) ([]byte, error) {
	var length int
	switch lengthSize {
	case 1:
		v, err := r.ReadUint8()
		if err != nil {
			return nil, err
		}
		length = int(v)
	case 2:
		v, err := r.ReadUint16()
		if err != nil {
			return nil, err
		}
		length = int(v)
	case 4:
		v, err := r.ReadUint32()
		if err != nil {
			return nil, err
		}
		length = int(v)
	default:
		return nil, fmt.Errorf("invalid length size: %d (must be 1, 2, or 4)", lengthSize)
	}
	return r.ReadBytes(length)
}

// ReadString 读取一个字符串
//
// 字符串格式：4 字节长度（uint32）+ UTF-8 编码的字符串数据
// 使用 string() 进行安全转换，避免使用 unsafe 包
func (r *Reader) ReadString() (string, error) {
	data, err := r.ReadBytesWithLength(LengthSize4)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadShortString 读取一个短字符串
//
// 字符串格式：1 字节长度（uint8）+ UTF-8 编码的字符串数据
// 适用于长度不超过 255 的字符串
func (r *Reader) ReadShortString() (string, error) {
	data, err := r.ReadBytesWithLength(LengthSize1)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Read 通用读取方法，支持任意基本类型和复杂类型
//
// v 必须是一个非 nil 指针，指向要读取的目标变量
// 支持的类型包括：
//   - 基本整数类型：*byte, *uint8, *int8, *uint16, *int16, *uint32, *int32, *uint64, *int64
//   - 浮点类型：*float32, *float64
//   - 布尔类型：*bool
//   - 字符串：*string
//   - 字节切片：*[]byte
//   - 复杂类型：通过反射支持切片和结构体（需要传递指针）
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
		val, err := r.ReadBytesWithLength(LengthSize4)
		if err != nil {
			return err
		}
		*ptr = val
	default:
		return r.readReflect(v)
	}
	return nil
}

// readReflect 使用反射机制读取复杂类型
//
// 支持的类型：
//   - 切片：先读取长度（uint32），然后递归读取每个元素
//   - 数组：直接读取所有元素（数组长度固定，不需要读取长度）
//   - 结构体：按字段顺序递归读取所有可导出字段
func (r *Reader) readReflect(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("must pass a non-nil pointer, got %T", v)
	}

	rv = rv.Elem()
	switch rv.Kind() {
	case reflect.Slice:
		// 读取切片长度
		length, err := r.ReadUint32()
		if err != nil {
			return err
		}

		// 创建切片并读取每个元素
		slice := reflect.MakeSlice(rv.Type(), int(length), int(length))
		for i := 0; i < int(length); i++ {
			elem := slice.Index(i)
			if elem.CanAddr() {
				if err := r.Read(elem.Addr().Interface()); err != nil {
					return err
				}
			} else {
				// 对于不可寻址的元素，创建临时变量
				elemPtr := reflect.New(rv.Type().Elem())
				if err := r.Read(elemPtr.Interface()); err != nil {
					return err
				}
				elem.Set(elemPtr.Elem())
			}
		}
		rv.Set(slice)
		return nil

	case reflect.Array:
		// 先读入临时数组，成功后再赋给目标，避免解码中途失败时污染调用者
		tmp := reflect.New(rv.Type()).Elem()
		expectedLength := tmp.Len()
		readLength, err := r.ReadUint32()
		if err != nil {
			return err
		}
		if int(readLength) != expectedLength {
			return fmt.Errorf("array length mismatch: expected %d, got %d", expectedLength, readLength)
		}
		for i := 0; i < expectedLength; i++ {
			elem := tmp.Index(i)
			if elem.CanAddr() {
				if err := r.Read(elem.Addr().Interface()); err != nil {
					return err
				}
			} else {
				elemPtr := reflect.New(tmp.Type().Elem())
				if err := r.Read(elemPtr.Interface()); err != nil {
					return err
				}
				elem.Set(elemPtr.Elem())
			}
		}
		rv.Set(tmp)
		return nil

	case reflect.Struct:
		// 先读入临时结构体，成功后再赋给目标，避免解码中途失败时污染调用者
		tmp := reflect.New(rv.Type()).Elem()
		for i := 0; i < tmp.NumField(); i++ {
			field := tmp.Field(i)
			if field.CanAddr() && field.Addr().CanInterface() {
				if err := r.Read(field.Addr().Interface()); err != nil {
					return err
				}
			}
		}
		rv.Set(tmp)
		return nil

	default:
		return fmt.Errorf("unsupported type for reading: %v", rv.Type())
	}
}

// ReadInto 一次性读取多个值
//
// vals 是要读取的变量指针列表，按顺序依次读取
// 如果任何读取操作失败，会立即返回错误
func (r *Reader) ReadInto(vals ...interface{}) error {
	for i, v := range vals {
		if err := r.Read(v); err != nil {
			return fmt.Errorf("read index %d failed: %w", i, err)
		}
	}
	return nil
}

// ReadMessage 从当前位置读取一条消息。
// 线格式：| 4 字节 body 长度 | body | 4 字节消息名长度 | 消息名 |。
// 解码失败时返回 (nil, err)，且不会修改调用者已传入的其它变量；内部会先读完 body/name 再解码，故流位置会正确前移。
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

// Remaining 获取剩余未读取的数据
//
// 返回的切片直接引用底层缓冲区，修改它可能会影响 Reader 的行为
// 如果 Reader 已发生错误，返回 nil
func (r *Reader) Remaining() []byte {
	if r.err != nil {
		return nil
	}
	return r.buf[r.pos:]
}

// RemainingSize 获取剩余未读取数据的字节数
func (r *Reader) RemainingSize() int {
	if r.err != nil {
		return 0
	}
	return len(r.buf) - r.pos
}

// Pos 返回当前读取位置（字节偏移量）
func (r *Reader) Pos() int {
	return r.pos
}

// Seek 将读取位置移动到指定偏移量
//
// pos 是目标位置，必须在 [0, len(buf)] 范围内
// 成功调用会清除之前的错误状态
func (r *Reader) Seek(pos int) error {
	if pos < 0 || pos > len(r.buf) {
		return fmt.Errorf("seek position %d out of range [0, %d]", pos, len(r.buf))
	}
	r.pos = pos
	r.err = nil // 清除之前的错误
	return nil
}

// Skip 跳过指定数量的字节
//
// n 是要跳过的字节数
// 如果剩余数据不足，会返回错误
func (r *Reader) Skip(n int) error {
	if !r.check(n) {
		return r.err
	}
	r.pos += n
	return nil
}

// Error 返回读取过程中遇到的第一个错误
//
// 一旦发生错误，后续的所有读取操作都会失败并返回相同的错误
func (r *Reader) Error() error {
	return r.err
}

// Reset 重置读取器，使用新的数据源
//
// data 是新的数据缓冲区
// 重置后，读取位置回到 0，错误状态被清除
func (r *Reader) Reset(data []byte) {
	r.buf = data
	r.pos = 0
	r.err = nil
}
