package envelope

import (
	"github.com/kercylan98/vivid/pkg/vivid"
)

func init() {
	vivid.GetMessageRegister().RegisterName("_vivid.core.envelope", new(envelope))
}

var _ vivid.Envelope = (*envelope)(nil)

type envelope struct {
	SenderID    vivid.ID
	ReceiverID  vivid.ID
	Message     vivid.Message
	MessageType vivid.MessageType
}

func (e *envelope) GetSender() vivid.ID {
	return e.SenderID
}

func (e *envelope) GetReceiver() vivid.ID {
	return e.ReceiverID
}

func (e *envelope) GetMessage() vivid.Message {
	return e.Message
}

func (e *envelope) GetMessageType() vivid.MessageType {
	return e.MessageType
}
