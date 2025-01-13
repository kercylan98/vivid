package envelope

import "github.com/kercylan98/vivid/core"

var _ core.Envelope = (*envelope)(nil)

type envelope struct {
	SenderID    core.ID
	ReceiverID  core.ID
	Message     core.Message
	MessageType core.MessageType
}

func (e *envelope) GetSender() core.ID {
	return e.SenderID
}

func (e *envelope) GetReceiver() core.ID {
	return e.ReceiverID
}

func (e *envelope) GetMessage() core.Message {
	return e.Message
}

func (e *envelope) GetMessageType() core.MessageType {
	return e.MessageType
}
