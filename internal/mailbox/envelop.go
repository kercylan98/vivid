package mailbox

import "github.com/kercylan98/vivid"

var (
	_ vivid.Envelop = (*Envelop)(nil)
)

func NewEnvelopWithTell(message vivid.Message) *Envelop {
	return &Envelop{
		message: message,
	}
}

func NewEnvelopWithAsk(agent EnvelopAgent, sender vivid.ActorRef, message vivid.Message) *Envelop {
	return &Envelop{
		agent:   agent,
		sender:  sender,
		message: message,
	}
}

type Envelop struct {
	agent   EnvelopAgent
	sender  vivid.ActorRef
	message vivid.Message
}

func (e *Envelop) Sender() vivid.ActorRef {
	return e.sender
}

func (e *Envelop) Message() vivid.Message {
	return e.message
}
