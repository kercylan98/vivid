package envelope

import (
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
)

var (
	_builder vivid2.EnvelopeBuilder = &builder{}
)

func Builder() vivid2.EnvelopeBuilder {
	return _builder
}

type builder struct{}

func (b *builder) EmptyOf() vivid2.Envelope {
	return &envelope{}
}

func (b *builder) Build(senderID vivid2.ID, receiverID vivid2.ID, message vivid2.Message, messageType vivid2.MessageType) vivid2.Envelope {
	return &envelope{
		SenderID:    senderID,
		ReceiverID:  receiverID,
		Message:     message,
		MessageType: messageType,
	}
}
