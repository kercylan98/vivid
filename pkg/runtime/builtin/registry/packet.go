package registry

import (
	"encoding/binary"
	"errors"
	"sync"

	"github.com/kercylan98/vivid/pkg/runtime"
)

const (
	flagSystemMessage = uint8(1 << 0) // bit0 标识系统消息
)

var packetPool = sync.Pool{
	New: func() any {
		return &Packet{}
	},
}

// AcquirePacket 从对象池获取一个 Packet 实例。
func AcquirePacket() *Packet {
	return packetPool.Get().(*Packet)
}

// ReleasePacket 将 Packet 实例归还到对象池。
func ReleasePacket(p *Packet) {
	p.version = 0
	p.flags = 0
	p.sequence = 0
	p.sender = ""
	p.receiver = ""
	p.messageName = ""
	p.messageData = nil
	packetPool.Put(p)
}

// Packet 表示网络传输的数据包。
// 协议格式:
// | version(uint8) | flags(uint8) | sequence(uint64) | senderLen(uint16) | sender(bytes) |
// | receiverLen(uint16) | receiver(bytes) | typeLen(uint8) | type(bytes) |
// | nameLen(uint16) | name(bytes) | dataLen(uint32) | data(bytes) |
type Packet struct {
	version     uint8  // 协议版本
	flags       uint8  // 标志位
	sequence    uint64 // 序列号
	sender      string // 发送者路径
	receiver    string // 接收者路径
	messageName string // 消息名称
	messageData []byte // 消息数据
}

// NewPacket 创建一个新的数据包。
func NewPacket(sequence uint64, sender, receiver, messageName string, messageData []byte, isSystem bool) *Packet {
	p := AcquirePacket()
	p.version = protocolVersion
	p.flags = 0
	if isSystem {
		p.flags |= flagSystemMessage
	}
	p.sequence = sequence
	p.sender = sender
	p.receiver = receiver
	p.messageName = messageName
	p.messageData = messageData
	return p
}

// Version 返回协议版本。
func (p *Packet) Version() uint8 {
	return p.version
}

// Sequence 返回序列号。
func (p *Packet) Sequence() uint64 {
	return p.sequence
}

// Sender 返回发送者路径。
func (p *Packet) Sender() string {
	return p.sender
}

// Receiver 返回接收者路径。
func (p *Packet) Receiver() string {
	return p.receiver
}

// MessageName 返回消息名称。
func (p *Packet) MessageName() string {
	return p.messageName
}

// MessageData 返回消息数据。
func (p *Packet) MessageData() []byte {
	return p.messageData
}

// IsSystemMessage 判断是否为系统消息。
func (p *Packet) IsSystemMessage() bool {
	return (p.flags & flagSystemMessage) != 0
}

// MessageType 返回 runtime 消息类型。
func (p *Packet) MessageType() runtime.MessageType {
	if p.IsSystemMessage() {
		return runtime.MessageTypeSystem
	}
	return runtime.MessageTypeUser
}

// Encode 将数据包编码为字节数组。
func (p *Packet) Encode() ([]byte, error) {
	senderBytes := []byte(p.sender)
	receiverBytes := []byte(p.receiver)
	nameBytes := []byte(p.messageName)

	// 检查长度限制
	if len(senderBytes) > 0xFFFF {
		return nil, errors.New("sender path too long")
	}
	if len(receiverBytes) > 0xFFFF {
		return nil, errors.New("receiver path too long")
	}
	if len(nameBytes) > 0xFFFF {
		return nil, errors.New("message name too long")
	}
	if len(p.messageData) > 0xFFFFFFFF {
		return nil, errors.New("message data too long")
	}

	// 计算总大小
	// 1(version) + 1(flags) + 8(sequence) + 2(senderLen) + len(sender)
	// + 2(receiverLen) + len(receiver) + 2(nameLen) + len(name) + 4(dataLen) + len(data)
	totalSize := 1 + 1 + 8 + 2 + len(senderBytes) + 2 + len(receiverBytes) +
		2 + len(nameBytes) + 4 + len(p.messageData)

	buf := make([]byte, totalSize)
	offset := 0

	// 写入版本
	buf[offset] = p.version
	offset++

	// 写入标志位
	buf[offset] = p.flags
	offset++

	// 写入序列号
	binary.BigEndian.PutUint64(buf[offset:], p.sequence)
	offset += 8

	// 写入发送者
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(senderBytes)))
	offset += 2
	copy(buf[offset:], senderBytes)
	offset += len(senderBytes)

	// 写入接收者
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(receiverBytes)))
	offset += 2
	copy(buf[offset:], receiverBytes)
	offset += len(receiverBytes)

	// 写入消息名称
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(nameBytes)))
	offset += 2
	copy(buf[offset:], nameBytes)
	offset += len(nameBytes)

	// 写入消息数据
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(p.messageData)))
	offset += 4
	copy(buf[offset:], p.messageData)

	return buf, nil
}

// Decode 从字节数组解码数据包。
func (p *Packet) Decode(data []byte) error {
	// 最小长度: 1(version) + 1(flags) + 8(sequence) + 2(senderLen) + 2(receiverLen)
	// + 2(nameLen) + 4(dataLen) = 21
	if len(data) < 21 {
		return errors.New("invalid packet: data too short")
	}

	offset := 0

	// 读取版本
	p.version = data[offset]
	offset++

	// 读取标志位
	p.flags = data[offset]
	offset++

	// 读取序列号
	p.sequence = binary.BigEndian.Uint64(data[offset:])
	offset += 8

	// 读取发送者
	senderLen := binary.BigEndian.Uint16(data[offset:])
	offset += 2
	if offset+int(senderLen) > len(data) {
		return errors.New("invalid packet: sender length out of bounds")
	}
	if senderLen > 0 {
		p.sender = string(data[offset : offset+int(senderLen)])
		offset += int(senderLen)
	} else {
		p.sender = ""
	}

	// 读取接收者
	if offset+2 > len(data) {
		return errors.New("invalid packet: receiver length out of bounds")
	}
	receiverLen := binary.BigEndian.Uint16(data[offset:])
	offset += 2
	if offset+int(receiverLen) > len(data) {
		return errors.New("invalid packet: receiver out of bounds")
	}
	if receiverLen > 0 {
		p.receiver = string(data[offset : offset+int(receiverLen)])
		offset += int(receiverLen)
	} else {
		p.receiver = ""
	}

	// 读取消息名称
	if offset+2 > len(data) {
		return errors.New("invalid packet: name length out of bounds")
	}
	nameLen := binary.BigEndian.Uint16(data[offset:])
	offset += 2
	if offset+int(nameLen) > len(data) {
		return errors.New("invalid packet: name out of bounds")
	}
	if nameLen > 0 {
		p.messageName = string(data[offset : offset+int(nameLen)])
		offset += int(nameLen)
	} else {
		p.messageName = ""
	}

	// 读取消息数据
	if offset+4 > len(data) {
		return errors.New("invalid packet: data length out of bounds")
	}
	dataLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(dataLen) > len(data) {
		return errors.New("invalid packet: data out of bounds")
	}
	if dataLen > 0 {
		p.messageData = data[offset : offset+int(dataLen)]
	} else {
		p.messageData = nil
	}

	return nil
}
