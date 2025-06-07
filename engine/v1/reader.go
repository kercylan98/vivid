package vivid

import (
	"encoding/binary"
	"math"
)

// reader 内部消息读取器
type reader struct {
	buf    []byte
	offset int
}

// newReader 创建新的读取器
func newReader(data []byte) *reader {
	return &reader{
		buf:    data,
		offset: 0,
	}
}

// checkRead 检查是否有足够的字节可读
func (r *reader) checkRead(size int) bool {
	return r.offset+size <= len(r.buf)
}

// ReadUint8 读取uint8（失败返回零值）
func (r *reader) readUint8() uint8 {
	if !r.checkRead(1) {
		return 0
	}
	value := r.buf[r.offset]
	r.offset++
	return value
}

// ReadUint16 读取uint16（小端序，失败返回零值）
func (r *reader) readUint16() uint16 {
	if !r.checkRead(2) {
		return 0
	}
	value := binary.LittleEndian.Uint16(r.buf[r.offset : r.offset+2])
	r.offset += 2
	return value
}

// ReadUint32 读取uint32（小端序，失败返回零值）
func (r *reader) readUint32() uint32 {
	if !r.checkRead(4) {
		return 0
	}
	value := binary.LittleEndian.Uint32(r.buf[r.offset : r.offset+4])
	r.offset += 4
	return value
}

// ReadUint64 读取uint64（小端序，失败返回零值）
func (r *reader) readUint64() uint64 {
	if !r.checkRead(8) {
		return 0
	}
	value := binary.LittleEndian.Uint64(r.buf[r.offset : r.offset+8])
	r.offset += 8
	return value
}

// ReadInt8 读取int8（失败返回零值）
func (r *reader) readInt8() int8 {
	return int8(r.readUint8())
}

// ReadInt16 读取int16（小端序，失败返回零值）
func (r *reader) readInt16() int16 {
	return int16(r.readUint16())
}

// ReadInt32 读取int32（小端序，失败返回零值）
func (r *reader) readInt32() int32 {
	return int32(r.readUint32())
}

// ReadInt64 读取int64（小端序，失败返回零值）
func (r *reader) readInt64() int64 {
	return int64(r.readUint64())
}

// ReadFloat32 读取float32（小端序，失败返回零值）
func (r *reader) readFloat32() float32 {
	bits := r.readUint32()
	return math.Float32frombits(bits)
}

// ReadFloat64 读取float64（小端序，失败返回零值）
func (r *reader) readFloat64() float64 {
	bits := r.readUint64()
	return math.Float64frombits(bits)
}

// ReadBool 读取bool（1字节，0或1，失败返回false）
func (r *reader) readBool() bool {
	value := r.readUint8()
	return value != 0
}

// ReadString 读取字符串（4字节长度 + 字符串内容，失败返回空字符串）
func (r *reader) readString() string {
	length := r.readUint32()

	if !r.checkRead(int(length)) {
		return ""
	}

	value := string(r.buf[r.offset : r.offset+int(length)])
	r.offset += int(length)
	return value
}

// ReadStrings 读取字符串数组（4字节长度 + 字符串内容，失败返回空数组）
func (r *reader) readStrings() []string {
	length := r.readUint32()

	value := make([]string, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readString()
	}
	return value
}

// ReadUint8s 读取uint8数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readUint8s() []uint8 {
	length := r.readUint32()

	if !r.checkRead(int(length)) {
		return nil
	}

	value := make([]uint8, length)
	copy(value, r.buf[r.offset:r.offset+int(length)])
	r.offset += int(length)
	return value
}

// ReadUint16s 读取uint16数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readUint16s() []uint16 {
	length := r.readUint32()

	value := make([]uint16, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readUint16()
	}
	return value
}

// ReadUint32s 读取uint32数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readUint32s() []uint32 {
	length := r.readUint32()

	value := make([]uint32, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readUint32()
	}
	return value
}

// ReadUint64s 读取uint64数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readUint64s() []uint64 {
	length := r.readUint32()

	value := make([]uint64, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readUint64()
	}
	return value
}

// ReadInt8s 读取int8数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readInt8s() []int8 {
	length := r.readUint32()

	value := make([]int8, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readInt8()
	}
	return value
}

// ReadInt16s 读取int16数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readInt16s() []int16 {
	length := r.readUint32()

	value := make([]int16, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readInt16()
	}
	return value
}

// ReadInt32s 读取int32数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readInt32s() []int32 {
	length := r.readUint32()

	value := make([]int32, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readInt32()
	}
	return value
}

// ReadInt64s 读取int64数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readInt64s() []int64 {
	length := r.readUint32()

	value := make([]int64, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readInt64()
	}
	return value
}

// ReadFloat32s 读取float32数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readFloat32s() []float32 {
	length := r.readUint32()

	value := make([]float32, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readFloat32()
	}
	return value
}

// ReadFloat64s 读取float64数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readFloat64s() []float64 {
	length := r.readUint32()

	value := make([]float64, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readFloat64()
	}
	return value
}

// ReadBools 读取bool数组（4字节长度 + 数据内容，失败返回空数组）
func (r *reader) readBools() []bool {
	length := r.readUint32()

	value := make([]bool, length)
	for i := 0; i < int(length); i++ {
		value[i] = r.readBool()
	}
	return value
}

// ReadBytes 读取字节数组（4字节长度 + 字节内容，失败返回空切片）
func (r *reader) readBytes() []byte {
	length := r.readUint32()

	if !r.checkRead(int(length)) {
		return nil
	}

	value := make([]byte, length)
	copy(value, r.buf[r.offset:r.offset+int(length)])
	r.offset += int(length)
	return value
}

// Remaining 返回剩余可读字节数
func (r *reader) Remaining() int {
	return len(r.buf) - r.offset
}

// Reset 重置读取器到初始位置
func (r *reader) Reset() {
	r.offset = 0
}

// 链式调用方法 - 读取到指针（失败时不改变原值）

// ReadUint8To 读取uint8到指针（链式调用）
func (r *reader) readUint8To(ptr *uint8) *reader {
	if r.checkRead(1) {
		*ptr = r.readUint8()
	}
	return r
}

// ReadUint16To 读取uint16到指针（链式调用）
func (r *reader) readUint16To(ptr *uint16) *reader {
	if r.checkRead(2) {
		*ptr = r.readUint16()
	}
	return r
}

// ReadUint32To 读取uint32到指针（链式调用）
func (r *reader) readUint32To(ptr *uint32) *reader {
	if r.checkRead(4) {
		*ptr = r.readUint32()
	}
	return r
}

// ReadUint64To 读取uint64到指针（链式调用）
func (r *reader) readUint64To(ptr *uint64) *reader {
	if r.checkRead(8) {
		*ptr = r.readUint64()
	}
	return r
}

// ReadInt8To 读取int8到指针（链式调用）
func (r *reader) readInt8To(ptr *int8) *reader {
	if r.checkRead(1) {
		*ptr = r.readInt8()
	}
	return r
}

// ReadInt16To 读取int16到指针（链式调用）
func (r *reader) readInt16To(ptr *int16) *reader {
	if r.checkRead(2) {
		*ptr = r.readInt16()
	}
	return r
}

// ReadInt32To 读取int32到指针（链式调用）
func (r *reader) readInt32To(ptr *int32) *reader {
	if r.checkRead(4) {
		*ptr = r.readInt32()
	}
	return r
}

// ReadInt64To 读取int64到指针（链式调用）
func (r *reader) readInt64To(ptr *int64) *reader {
	if r.checkRead(8) {
		*ptr = r.readInt64()
	}
	return r
}

// ReadFloat32To 读取float32到指针（链式调用）
func (r *reader) readFloat32To(ptr *float32) *reader {
	if r.checkRead(4) {
		*ptr = r.readFloat32()
	}
	return r
}

// ReadFloat64To 读取float64到指针（链式调用）
func (r *reader) readFloat64To(ptr *float64) *reader {
	if r.checkRead(8) {
		*ptr = r.readFloat64()
	}
	return r
}

// ReadBoolTo 读取bool到指针（链式调用）
func (r *reader) readBoolTo(ptr *bool) *reader {
	if r.checkRead(1) {
		*ptr = r.readBool()
	}
	return r
}

// ReadStringTo 读取字符串到指针（链式调用）
func (r *reader) readStringTo(ptr *string) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readString()
	}
	return r
}

// ReadStringsTo 读取字符串数组到指针（链式调用）
func (r *reader) readStringsTo(ptr *[]string) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readStrings()
	}
	return r
}

// ReadUint8sTo 读取uint8数组到指针（链式调用）
func (r *reader) readUint8sTo(ptr *[]uint8) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readUint8s()
	}
	return r
}

// ReadUint16sTo 读取uint16数组到指针（链式调用）
func (r *reader) readUint16sTo(ptr *[]uint16) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readUint16s()
	}
	return r
}

// ReadUint32sTo 读取uint32数组到指针（链式调用）
func (r *reader) readUint32sTo(ptr *[]uint32) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readUint32s()
	}
	return r
}

// ReadUint64sTo 读取uint64数组到指针（链式调用）
func (r *reader) readUint64sTo(ptr *[]uint64) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readUint64s()
	}
	return r
}

// ReadInt8sTo 读取int8数组到指针（链式调用）
func (r *reader) readInt8sTo(ptr *[]int8) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readInt8s()
	}
	return r
}

// ReadInt16sTo 读取int16数组到指针（链式调用）
func (r *reader) readInt16sTo(ptr *[]int16) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readInt16s()
	}
	return r
}

// ReadInt32sTo 读取int32数组到指针（链式调用）
func (r *reader) readInt32sTo(ptr *[]int32) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readInt32s()
	}
	return r
}

// ReadInt64sTo 读取int64数组到指针（链式调用）
func (r *reader) readInt64sTo(ptr *[]int64) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readInt64s()
	}
	return r
}

// ReadFloat32sTo 读取float32数组到指针（链式调用）
func (r *reader) readFloat32sTo(ptr *[]float32) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readFloat32s()
	}
	return r
}

// ReadFloat64sTo 读取float64数组到指针（链式调用）
func (r *reader) readFloat64sTo(ptr *[]float64) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readFloat64s()
	}
	return r
}

// ReadBoolsTo 读取bool数组到指针（链式调用）
func (r *reader) readBoolsTo(ptr *[]bool) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readBools()
	}
	return r
}

// ReadBytesTo 读取字节数组到指针（链式调用）
func (r *reader) readBytesTo(ptr *[]byte) *reader {
	if r.checkRead(4) { // 至少需要4字节来读取长度
		*ptr = r.readBytes()
	}
	return r
}

// readWith 以自定义方式读取数据
func (r *reader) readWith(h func(r *reader)) *reader {
	h(r)
	return r
}
