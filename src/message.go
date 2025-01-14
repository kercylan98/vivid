package vivid

const (
	// UserMessage 表示用户消息，该类型消息优先级将低于 SystemMessage
	UserMessage MessageType = iota
	// SystemMessage 表示系统消息，该类型消息优先级为最高
	SystemMessage
)

var (
	_                       Envelope        = (*defaultEnvelope)(nil)   // 确保 defaultEnvelope 实现了 Envelope 接口
	_defaultEnvelopeBuilder EnvelopeBuilder = &defaultEnvelopeBuilder{} // 默认的 EnvelopeBuilder 实现，并确保实现了 EnvelopeBuilder 接口
)

// MessageType 是消息的类型，它用于区分消息的优先级及执行方式
type MessageType = int8

func init() {
	RegisterMessageName("vivid.defaultEnvelope", &defaultEnvelope{})
}

// getEnvelopeBuilder 获取默认的 EnvelopeBuilder 实现，该函数在外部不应该被调用
func getEnvelopeBuilder() EnvelopeBuilder {
	return _defaultEnvelopeBuilder
}

// EnvelopeBuilder 是 Envelope 的构建器，由于 Envelope 支持不同的实现，且包含多种构建方式，因此需要通过构建器来进行创建
type EnvelopeBuilder interface {
	// Build 构建一个空的消息包装，它不包含任何头部信息及消息内容，适用于反序列化场景
	Build() Envelope

	// StandardOf 构建一个标准的消息包装，它包含了消息的发送者、接收者、消息内容及消息类型
	StandardOf(senderID ID, receiverID ID, messageType MessageType, message Message) Envelope

	// ConvertTypeOf 根据传入的消息包装构建一个新的消息包装，但消息类型将被替换为新的类型
	ConvertTypeOf(envelope Envelope, messageType MessageType) Envelope
}

// Envelope 是进程间通信的消息包装，包含原始消息内容和附加的头部信息，支持跨网络传输。
//   - 如果需要支持其他序列化方式，可以通过实现 Envelope 接口并自定义消息包装，同时实现 EnvelopeBuilder 接口来提供构建方式。
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

// defaultEnvelopeBuilder 是 EnvelopeBuilder 的默认实现，它提供了 defaultEnvelope 的构建方式
type defaultEnvelopeBuilder struct{}

func (d *defaultEnvelopeBuilder) Build() Envelope {
	return &defaultEnvelope{}
}

func (d *defaultEnvelopeBuilder) StandardOf(senderID ID, receiverID ID, messageType MessageType, message Message) Envelope {
	return &defaultEnvelope{
		Sender:      senderID,
		Receiver:    receiverID,
		Message:     message,
		MessageType: messageType,
	}
}

func (d *defaultEnvelopeBuilder) ConvertTypeOf(envelope Envelope, messageType MessageType) Envelope {
	return &defaultEnvelope{
		Sender:      envelope.GetSender(),
		Receiver:    envelope.GetReceiver(),
		Message:     envelope.GetMessage(),
		MessageType: messageType,
	}
}

// defaultEnvelope 是 Envelope 的默认实现，它基于 gob 序列化方式实现了 Envelope 接口
type defaultEnvelope struct {
	Sender      ID
	Receiver    ID
	Message     Message
	MessageType MessageType
}

func (d *defaultEnvelope) GetSender() ID {
	return d.Sender
}

func (d *defaultEnvelope) GetReceiver() ID {
	return d.Receiver
}

func (d *defaultEnvelope) GetMessage() Message {
	return d.Message
}

func (d *defaultEnvelope) GetMessageType() MessageType {
	return d.MessageType
}
