package envelope

import "github.com/kercylan98/vivid/core"

var (
	_builder core.EnvelopeBuilder = &builder{}
)

func Builder() core.EnvelopeBuilder {
	return _builder
}

type builder struct{}

func (b *builder) Build(senderID core.ID, receiverID core.ID, message core.Message, messageType core.MessageType) core.Envelope {
	return &envelope{
		senderID:    senderID,
		receiverID:  receiverID,
		message:     message,
		messageType: messageType,
	}
}
