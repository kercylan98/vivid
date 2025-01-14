package envelope

import (
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
)

func init() {
	vivid2.GetMessageRegister().RegisterName("_vivid.core.envelope", new(envelope))
}

var _ vivid2.Envelope = (*envelope)(nil)

type envelope struct {
	SenderID    vivid2.ID
	ReceiverID  vivid2.ID
	Message     vivid2.Message
	MessageType vivid2.MessageType
}

func (e *envelope) GetSender() vivid2.ID {
	return e.SenderID
}

func (e *envelope) GetReceiver() vivid2.ID {
	return e.ReceiverID
}

func (e *envelope) GetMessage() vivid2.Message {
	return e.Message
}

func (e *envelope) GetMessageType() vivid2.MessageType {
	return e.MessageType
}
