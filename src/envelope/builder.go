package envelope

import (
	"github.com/kercylan98/vivid/pkg/vivid"
)

var (
	_builder vivid.EnvelopeBuilder = &builder{}
)

func Builder() vivid.EnvelopeBuilder {
	return _builder
}

type builder struct{}

func (b *builder) EmptyOf() vivid.Envelope {
	return &envelope{}
}

func (b *builder) Build(senderID vivid.ID, receiverID vivid.ID, message vivid.Message, messageType vivid.MessageType) vivid.Envelope {
	return &envelope{
		SenderID:    senderID,
		ReceiverID:  receiverID,
		Message:     message,
		MessageType: messageType,
	}
}
