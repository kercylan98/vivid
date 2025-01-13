package core

type EnvelopeBuilder interface {
	// Build 构建一个消息包装
	Build(senderID ID, receiverID ID, message Message, messageType MessageType) Envelope

	// EmptyOf 构建一个空的消息包装，他仅在一些特殊场景使用
	EmptyOf() Envelope
}

// Envelope 是进程间通讯传输的消息包装，它提供了原始的消息内容及一些额外的头部信息
type Envelope interface {
	// GetSender 获取消息发送者的 ID
	GetSender() ID

	// GetReceiver 获取消息接收者的 ID
	GetReceiver() ID

	// GetMessage 获取消息的内容
	GetMessage() Message

	// GetMessageType 获取消息的类型
	GetMessageType() MessageType
}

type EnvelopeProvider interface {
	Provide() Envelope
}

type FnEnvelopeProvider func() Envelope

func (f FnEnvelopeProvider) Provide() Envelope {
	return f()
}
