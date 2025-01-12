package main

// Envelope 是进程间通讯传输的消息包装，它提供了原始的消息内容及一些额外的头部信息
type Envelope interface {
	// SenderID 获取消息发送者的 ID
	SenderID() ID

	// ReceiverID 获取消息接收者的 ID
	ReceiverID() ID

	// Message 获取消息的内容
	Message() Message

	// MessageType 获取消息的类型
	MessageType() MessageType
}
