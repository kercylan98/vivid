package core

import (
	"encoding/gob"
)

var (
	_                MessageRegister = (*messageRegister)(nil)
	_messageRegister MessageRegister = &messageRegister{}
)

// GetMessageRegister 获取消息注册器
//   - 当位于跨网络环境且使用的是 vivid 提供的默认服务器时，需要通过此方法注册你的消息类型
func GetMessageRegister() MessageRegister {
	return _messageRegister
}

type MessageRegister interface {
	Register(message Message)
	RegisterName(name string, message Message)
}

type messageRegister struct{}

func (m *messageRegister) Register(message Message) {
	gob.Register(message)
}

func (m *messageRegister) RegisterName(name string, message Message) {
	gob.RegisterName(name, message)
}
