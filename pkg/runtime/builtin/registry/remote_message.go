package registry

import (
	"encoding/binary"
	"errors"
	"sync"

	"github.com/kercylan98/vivid/pkg/runtime"
)

var remoteMessagePool = sync.Pool{
	New: func() any {
		return &remoteMessage{}
	},
}

func newRemoteMessageWithPool(sequence uint64, receiver *runtime.ProcessID, messageType runtime.MessageType, messageName string, messageBytes []byte) *remoteMessage {
	msg := remoteMessagePool.Get().(*remoteMessage)
	msg.sequence = sequence
	msg.receiver = receiver
	msg.messageType = messageType
	msg.messageName = messageName
	msg.messageBytes = messageBytes
	return msg
}

func putRemoteMessageToPool(msg *remoteMessage) {
	msg.receiver = nil
	msg.messageName = ""
	msg.messageBytes = nil
	remoteMessagePool.Put(msg)
}

type remoteMessage struct {
	sequence     uint64              // 序列号
	receiver     *runtime.ProcessID  // 接收者
	messageType  runtime.MessageType // 消息类型
	messageName  string              // 消息名称（用于根据名称构建消息实例）
	messageBytes []byte              // 消息字节（用于反序列化消息至实例）
}

func (m *remoteMessage) encode() ([]byte, error) {
	// codec: | sequence(uint64) | type(int8) | pathLen(uint16) | path(bytes) | nameLen(uint16) | name(bytes) | msgLen(uint32) | msg(bytes) |

	// 提取 receiver path
	var pathBytes []byte
	if m.receiver != nil {
		pathBytes = []byte(m.receiver.Path())
	}
	nameBytes := []byte(m.messageName)

	// 检查长度限制
	if len(pathBytes) > 0xFFFF {
		return nil, errors.New("receiver path too long")
	}
	if len(nameBytes) > 0xFFFF {
		return nil, errors.New("message name too long")
	}
	if len(m.messageBytes) > 0xFFFFFFFF {
		return nil, errors.New("message bytes too long")
	}

	// 预先计算总大小并一次性分配
	// 8(sequence) + 1(type) + 2(pathLen) + len(path) + 2(nameLen) + len(name) + 4(msgLen) + len(msg)
	totalSize := 8 + 1 + 2 + len(pathBytes) + 2 + len(nameBytes) + 4 + len(m.messageBytes)
	buf := make([]byte, totalSize)

	offset := 0

	// 写入 sequence
	binary.BigEndian.PutUint64(buf[offset:], m.sequence)
	offset += 8

	// 写入 type
	buf[offset] = byte(m.messageType)
	offset++

	// 写入 path
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(pathBytes)))
	offset += 2
	copy(buf[offset:], pathBytes)
	offset += len(pathBytes)

	// 写入消息名称
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(nameBytes)))
	offset += 2
	copy(buf[offset:], nameBytes)
	offset += len(nameBytes)

	// 写入消息体
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.messageBytes)))
	offset += 4
	copy(buf[offset:], m.messageBytes)

	return buf, nil
}

func (m *remoteMessage) decode(data []byte, localAddress string) error {
	if len(data) < 15 { // 最小长度: 8(sequence) + 1(type) + 2(pathLen) + 2(nameLen) + 4(msgLen)
		return errors.New("invalid message: data too short")
	}

	offset := 0

	// 读取 sequence
	m.sequence = binary.BigEndian.Uint64(data[offset:])
	offset += 8

	// 读取 type
	m.messageType = runtime.MessageType(int8(data[offset]))
	offset++

	// 读取 path
	pathLen := binary.BigEndian.Uint16(data[offset:])
	offset += 2
	if offset+int(pathLen) > len(data) {
		return errors.New("invalid message: path length out of bounds")
	}

	var path string
	if pathLen > 0 {
		path = string(data[offset : offset+int(pathLen)])
		offset += int(pathLen)
		// 使用本地 address 和接收到的 path 重建 ProcessID
		pid := runtime.NewProcessID(localAddress, path)
		m.receiver = &pid
	} else {
		m.receiver = nil
	}

	// 读取消息名称
	if offset+2 > len(data) {
		return errors.New("invalid message: name length out of bounds")
	}
	nameLen := binary.BigEndian.Uint16(data[offset:])
	offset += 2
	if offset+int(nameLen) > len(data) {
		return errors.New("invalid message: name out of bounds")
	}

	if nameLen > 0 {
		m.messageName = string(data[offset : offset+int(nameLen)])
		offset += int(nameLen)
	} else {
		m.messageName = ""
	}

	// 读取消息体
	if offset+4 > len(data) {
		return errors.New("invalid message: message bytes length out of bounds")
	}
	msgLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(msgLen) > len(data) {
		return errors.New("invalid message: message bytes out of bounds")
	}

	if msgLen > 0 {
		m.messageBytes = data[offset : offset+int(msgLen)]
	} else {
		m.messageBytes = nil
	}

	return nil
}
