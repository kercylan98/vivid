package vivid

import (
	"encoding/binary"
	"math"
)

// writer 消息写入器
type writer struct {
	buf []byte
}

// newWriter 创建新的消息写入器
func newWriter() *writer {
	return &writer{
		buf: make([]byte, 0, 256), // 预分配256字节
	}
}

// newWriterCapacity 创建新的消息写入器，指定缓冲区容量
func newWriterCapacity(capacity int) *writer {
	return &writer{
		buf: make([]byte, 0, capacity),
	}
}

// WriteUint8 写入uint8（支持链式调用）
func (w *writer) writeUint8(value uint8) *writer {
	w.buf = append(w.buf, value)
	return w
}

// WriteUint16 写入uint16（小端序，支持链式调用）
func (w *writer) writeUint16(value uint16) *writer {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, value)
	w.buf = append(w.buf, b...)
	return w
}

// WriteUint32 写入uint32（小端序，支持链式调用）
func (w *writer) writeUint32(value uint32) *writer {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, value)
	w.buf = append(w.buf, b...)
	return w
}

// WriteUint64 写入uint64（小端序，支持链式调用）
func (w *writer) writeUint64(value uint64) *writer {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, value)
	w.buf = append(w.buf, b...)
	return w
}

// WriteInt8 写入int8（支持链式调用）
func (w *writer) writeInt8(value int8) *writer {
	return w.writeUint8(uint8(value))
}

// WriteInt16 写入int16（小端序，支持链式调用）
func (w *writer) writeInt16(value int16) *writer {
	return w.writeUint16(uint16(value))
}

// WriteInt32 写入int32（小端序，支持链式调用）
func (w *writer) writeInt32(value int32) *writer {
	return w.writeUint32(uint32(value))
}

// WriteInt64 写入int64（小端序，支持链式调用）
func (w *writer) writeInt64(value int64) *writer {
	return w.writeUint64(uint64(value))
}

// WriteFloat32 写入float32（小端序，支持链式调用）
func (w *writer) writeFloat32(value float32) *writer {
	return w.writeUint32(math.Float32bits(value))
}

// WriteFloat64 写入float64（小端序，支持链式调用）
func (w *writer) writeFloat64(value float64) *writer {
	return w.writeUint64(math.Float64bits(value))
}

// WriteBool 写入bool（1字节，0或1，支持链式调用）
func (w *writer) writeBool(value bool) *writer {
	if value {
		return w.writeUint8(1)
	} else {
		return w.writeUint8(0)
	}
}

// WriteString 写入字符串（4字节长度 + 字符串内容，支持链式调用）
func (w *writer) writeString(value string) *writer {
	data := []byte(value)
	w.writeUint32(uint32(len(data)))
	w.buf = append(w.buf, data...)
	return w
}

// WriteBytes 写入字节数组（4字节长度 + 字节内容，支持链式调用）
func (w *writer) writeBytes(value []byte) *writer {
	w.writeUint32(uint32(len(value)))
	w.buf = append(w.buf, value...)
	return w
}

// WriteStrings 写入字符串数组（4字节长度 + 字符串内容，支持链式调用）
func (w *writer) writeStrings(value []string) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeString(v)
	}
	return w
}

// WriteUint8s 写入uint8数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeUint8s(value []uint8) *writer {
	w.writeUint32(uint32(len(value)))
	w.buf = append(w.buf, value...)
	return w
}

// WriteUint16s 写入uint16数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeUint16s(value []uint16) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeUint16(v)
	}
	return w
}

// WriteUint32s 写入uint32数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeUint32s(value []uint32) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeUint32(v)
	}
	return w
}

// WriteUint64s 写入uint64数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeUint64s(value []uint64) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeUint64(v)
	}
	return w
}

// WriteInt8s 写入int8数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeInt8s(value []int8) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeInt8(v)
	}
	return w
}

// WriteInt16s 写入int16数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeInt16s(value []int16) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeInt16(v)
	}
	return w
}

// WriteInt32s 写入int32数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeInt32s(value []int32) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeInt32(v)
	}
	return w
}

// WriteInt64s 写入int64数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeInt64s(value []int64) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeInt64(v)
	}
	return w
}

// WriteFloat32s 写入float32数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeFloat32s(value []float32) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeFloat32(v)
	}
	return w
}

// WriteFloat64s 写入float64数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeFloat64s(value []float64) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeFloat64(v)
	}
	return w
}

// WriteBools 写入bool数组（4字节长度 + 数据内容，支持链式调用）
func (w *writer) writeBools(value []bool) *writer {
	w.writeUint32(uint32(len(value)))
	for _, v := range value {
		w.writeBool(v)
	}
	return w
}

// bytes 获取写入的字节数据
func (w *writer) bytes() []byte {
	return w.buf
}

// Reset 重置写入器
func (w *writer) Reset() {
	w.buf = w.buf[:0]
}
