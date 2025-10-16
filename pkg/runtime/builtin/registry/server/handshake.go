package server

import (
	"encoding/binary"
	"errors"
)

const (
	protocolVersion = uint8(1) // 协议版本号
)

// Handshake 表示握手协议消息。
// 握手协议用于在连接建立后交换节点信息。
type Handshake struct {
	version uint8  // 协议版本
	address string // 节点地址
}

// NewHandshake 创建一个新的握手消息。
func NewHandshake(address string) *Handshake {
	return &Handshake{
		version: protocolVersion,
		address: address,
	}
}

// Version 返回协议版本号。
func (h *Handshake) Version() uint8 {
	return h.version
}

// Address 返回节点地址。
func (h *Handshake) Address() string {
	return h.address
}

// Encode 将握手消息编码为字节数组。
// 协议格式: | version(uint8) | addressLen(uint16) | address(bytes) |
func (h *Handshake) Encode() ([]byte, error) {
	addressBytes := []byte(h.address)
	if len(addressBytes) > 0xFFFF {
		return nil, errors.New("address too long")
	}

	// 预先计算总大小: 1(version) + 2(addressLen) + len(address)
	totalSize := 1 + 2 + len(addressBytes)
	buf := make([]byte, totalSize)

	offset := 0

	// 写入版本号
	buf[offset] = h.version
	offset++

	// 写入地址长度
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(addressBytes)))
	offset += 2

	// 写入地址
	copy(buf[offset:], addressBytes)

	return buf, nil
}

// Decode 从字节数组解码握手消息。
func (h *Handshake) Decode(data []byte) error {
	if len(data) < 3 { // 最小长度: 1(version) + 2(addressLen)
		return errors.New("invalid handshake: data too short")
	}

	offset := 0

	// 读取版本号
	h.version = data[offset]
	offset++

	// 读取地址长度
	addressLen := binary.BigEndian.Uint16(data[offset:])
	offset += 2

	if offset+int(addressLen) > len(data) {
		return errors.New("invalid handshake: address length out of bounds")
	}

	// 读取地址
	if addressLen > 0 {
		h.address = string(data[offset : offset+int(addressLen)])
	} else {
		h.address = ""
	}

	return nil
}
