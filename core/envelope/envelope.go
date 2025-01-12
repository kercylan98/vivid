package envelope

import "github.com/kercylan98/vivid/core"

var _ core.Envelope = (*envelope)(nil)

type envelope struct {
	senderID    core.ID
	receiverID  core.ID
	message     core.Message
	messageType core.MessageType
}

func (e *envelope) SenderID() core.ID {
	return e.senderID
}

func (e *envelope) ReceiverID() core.ID {
	return e.receiverID
}

func (e *envelope) Message() core.Message {
	return e.message
}

func (e *envelope) MessageType() core.MessageType {
	return e.messageType
}
