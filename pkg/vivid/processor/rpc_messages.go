package processor

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/kercylan98/vivid/pkg/serializer"
)

var (
	_ RPCHandshake = (*rpcHandshake)(nil)
)

// RPCHandshake 定义RPC握手消息接口
type RPCHandshake interface {
	serializer.MarshalerUnmarshaler

	GetAddress() string
}

// rpcHandshake 握手消息实现
type rpcHandshake struct {
	address string
}

// NewRPCHandshake 创建新的握手消息实例
func NewRPCHandshake() RPCHandshake {
	return &rpcHandshake{}
}

// NewRPCHandshakeFromBytes 从字节流反序列化握手消息
func NewRPCHandshakeFromBytes(data []byte) (RPCHandshake, error) {
	m := &rpcHandshake{}
	if err := m.Unmarshal(data); err != nil {
		return nil, fmt.Errorf("unmarshal handshake failed: %w", err)
	}
	return m, nil
}

// NewRPCHandshakeWithAddress 使用地址创建握手消息
func NewRPCHandshakeWithAddress(address string) RPCHandshake {
	return &rpcHandshake{address: address}
}

// Marshal 序列化握手消息
func (m *rpcHandshake) Marshal() ([]byte, error) {
	addrBytes := []byte(m.address)
	buf := make([]byte, 2+len(addrBytes))
	binary.BigEndian.PutUint16(buf[:2], uint16(len(addrBytes)))
	copy(buf[2:], addrBytes)
	return buf, nil
}

// Unmarshal 反序列化握手消息
func (m *rpcHandshake) Unmarshal(data []byte) error {
	if len(data) < 2 {
		return errors.New("handshake data too short (missing length prefix)")
	}

	addrLen := binary.BigEndian.Uint16(data[:2])
	if len(data) < 2+int(addrLen) {
		return fmt.Errorf("handshake address data incomplete (need %d bytes, got %d)", 2+int(addrLen), len(data))
	}

	m.address = string(data[2 : 2+int(addrLen)])
	return nil
}

// GetAddress 获取地址
func (m *rpcHandshake) GetAddress() string {
	return m.address
}

// RPCBatchMessage 定义批量RPC消息接口
type RPCBatchMessage interface {
	serializer.MarshalerUnmarshaler

	Add(sender, target, name string, message []byte, system bool)
	Len() int
	Get(index int) (sender, target, name string, message []byte, system bool)
}

// rpcBatchMessage 批量消息实现
type rpcBatchMessage struct {
	names    []string
	messages [][]byte
	senders  []string
	targets  []string
	systems  []bool
}

// NewRPCBatchMessage 创建新的批量消息实例
func NewRPCBatchMessage() RPCBatchMessage {
	return &rpcBatchMessage{}
}

// Add 添加批量消息条目
func (r *rpcBatchMessage) Add(sender, target, name string, message []byte, system bool) {
	r.senders = append(r.senders, sender)
	r.targets = append(r.targets, target)
	r.names = append(r.names, name)
	r.messages = append(r.messages, message)
	r.systems = append(r.systems, system)
}

// Len 获取条目数量
func (r *rpcBatchMessage) Len() int {
	return len(r.names)
}

// Get 获取指定位置的条目
func (r *rpcBatchMessage) Get(index int) (address, path, name string, message []byte, system bool) {
	if index < 0 || index >= r.Len() {
		panic("index out of range")
	}
	return r.senders[index], r.targets[index], r.names[index], r.messages[index], r.systems[index]
}

// Marshal 序列化批量消息（优化版：预分配内存）
func (r *rpcBatchMessage) Marshal() ([]byte, error) {
	// 预计算各部分长度
	namesPart := r.calculateStringSliceLength(r.names)
	messagesPart := r.calculateBytesSliceLength(r.messages)
	sendersPart := r.calculateStringSliceLength(r.senders)
	targetsPart := r.calculateStringSliceLength(r.targets)
	systemsPart := r.calculateBoolSliceLength(r.systems)

	totalLen := 2 + namesPart + 2 + messagesPart + 2 + sendersPart + 2 + targetsPart + 2 + systemsPart
	buf := make([]byte, 0, totalLen)

	// 序列化各部分
	buf = r.appendLengthPrefixed(buf, uint16(r.Len()), func(buf []byte) []byte {
		for _, name := range r.names {
			buf = r.appendString(buf, name)
		}
		return buf
	})

	buf = r.appendLengthPrefixed(buf, uint16(len(r.messages)), func(buf []byte) []byte {
		for _, msg := range r.messages {
			buf = r.appendBytes(buf, msg)
		}
		return buf
	})

	buf = r.appendLengthPrefixed(buf, uint16(len(r.senders)), func(buf []byte) []byte {
		for _, addr := range r.senders {
			buf = r.appendString(buf, addr)
		}
		return buf
	})

	buf = r.appendLengthPrefixed(buf, uint16(len(r.targets)), func(buf []byte) []byte {
		for _, path := range r.targets {
			buf = r.appendString(buf, path)
		}
		return buf
	})

	buf = r.appendLengthPrefixed(buf, uint16(len(r.systems)), func(buf []byte) []byte {
		for _, sys := range r.systems {
			buf = r.appendBool(buf, sys)
		}
		return buf
	})

	return buf, nil
}

// Unmarshal 反序列化批量消息（优化版：提取公共解析逻辑）
func (r *rpcBatchMessage) Unmarshal(data []byte) error {
	offset := 0

	// 解析names
	if err := r.parseStringSlice(data, &offset, &r.names); err != nil {
		return fmt.Errorf("parse names failed: %w", err)
	}

	// 解析messages
	if err := r.parseBytesSlice(data, &offset, &r.messages); err != nil {
		return fmt.Errorf("parse messages failed: %w", err)
	}

	// 解析addresses
	if err := r.parseStringSlice(data, &offset, &r.senders); err != nil {
		return fmt.Errorf("parse senders failed: %w", err)
	}

	// 解析paths
	if err := r.parseStringSlice(data, &offset, &r.targets); err != nil {
		return fmt.Errorf("parse targets failed: %w", err)
	}

	// 解析systems
	if err := r.parseBoolSlice(data, &offset, &r.systems); err != nil {
		return fmt.Errorf("parse systems failed: %w", err)
	}

	// 校验总长度
	if offset != len(data) {
		return fmt.Errorf("data length mismatch (expected %d, got %d)", len(data), offset)
	}
	return nil
}

// 辅助方法：计算字符串切片序列化后的长度
func (r *rpcBatchMessage) calculateStringSliceLength(strings []string) int {
	length := 2 // 长度前缀
	for _, s := range strings {
		length += 2 + len(s) // 2字节长度 + 字符串内容
	}
	return length
}

// 辅助方法：计算字节切片序列化后的长度
func (r *rpcBatchMessage) calculateBytesSliceLength(bytes [][]byte) int {
	length := 2 // 长度前缀
	for _, b := range bytes {
		length += 4 + len(b) // 4字节长度 + 字节内容
	}
	return length
}

// 辅助方法：计算布尔切片序列化后的长度
func (r *rpcBatchMessage) calculateBoolSliceLength(bools []bool) int {
	return 2 + len(bools) // 2字节长度 + 1字节/布尔值
}

// 辅助方法：追加带长度前缀的数据块
func (r *rpcBatchMessage) appendLengthPrefixed(buf []byte, length uint16, appendFunc func([]byte) []byte) []byte {
	buf = append(buf, uint16ToBytes(length)...)
	buf = appendFunc(buf)
	return buf
}

// 辅助方法：追加字符串（带2字节长度前缀）
func (r *rpcBatchMessage) appendString(buf []byte, s string) []byte {
	buf = append(buf, uint16ToBytes(uint16(len(s)))...)
	buf = append(buf, s...)
	return buf
}

// 辅助方法：追加字节切片（带4字节长度前缀）
func (r *rpcBatchMessage) appendBytes(buf []byte, b []byte) []byte {
	buf = append(buf, uint32ToBytes(uint32(len(b)))...)
	buf = append(buf, b...)
	return buf
}

// 辅助方法：追加布尔值（1字节）
func (r *rpcBatchMessage) appendBool(buf []byte, b bool) []byte {
	if b {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	return buf
}

// 辅助方法：解析字符串切片
func (r *rpcBatchMessage) parseStringSlice(data []byte, offset *int, target *[]string) error {
	if *offset+2 > len(data) {
		return errors.New("invalid string slice length prefix")
	}
	count := int(binary.BigEndian.Uint16(data[*offset : *offset+2]))
	*offset += 2

	*target = make([]string, 0, count)
	for i := 0; i < count; i++ {
		if *offset+2 > len(data) {
			return fmt.Errorf("string %d missing length prefix", i)
		}
		strLen := int(binary.BigEndian.Uint16(data[*offset : *offset+2]))
		*offset += 2

		if *offset+strLen > len(data) {
			return fmt.Errorf("string %d data incomplete (need %d bytes, got %d)", i, strLen, len(data)-*offset)
		}
		*target = append(*target, string(data[*offset:*offset+strLen]))
		*offset += strLen
	}
	return nil
}

// 辅助方法：解析字节切片
func (r *rpcBatchMessage) parseBytesSlice(data []byte, offset *int, target *[][]byte) error {
	if *offset+2 > len(data) {
		return errors.New("invalid bytes slice length prefix")
	}
	count := int(binary.BigEndian.Uint16(data[*offset : *offset+2]))
	*offset += 2

	*target = make([][]byte, 0, count)
	for i := 0; i < count; i++ {
		if *offset+4 > len(data) {
			return fmt.Errorf("bytes %d missing length prefix", i)
		}
		msgLen := int(binary.BigEndian.Uint32(data[*offset : *offset+4]))
		*offset += 4

		if *offset+msgLen > len(data) {
			return fmt.Errorf("bytes %d data incomplete (need %d bytes, got %d)", i, msgLen, len(data)-*offset)
		}
		*target = append(*target, data[*offset:*offset+msgLen])
		*offset += msgLen
	}
	return nil
}

// 辅助方法：解析布尔切片
func (r *rpcBatchMessage) parseBoolSlice(data []byte, offset *int, target *[]bool) error {
	if *offset+2 > len(data) {
		return errors.New("invalid bool slice length prefix")
	}
	count := int(binary.BigEndian.Uint16(data[*offset : *offset+2]))
	*offset += 2

	*target = make([]bool, 0, count)
	for i := 0; i < count; i++ {
		if *offset >= len(data) {
			return fmt.Errorf("bool %d missing value", i)
		}
		*target = append(*target, data[*offset] == 1)
		*offset++
	}
	return nil
}

// 工具函数：uint16转大端字节数组
func uint16ToBytes(n uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, n)
	return b
}

// 工具函数：uint32转大端字节数组
func uint32ToBytes(n uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return b
}
