package actor

import "github.com/kercylan98/vivid"

var (
	_ vivid.Envelop = (*replacedEnvelop)(nil)
)

func newReplacedEnvelop(envelop vivid.Envelop, message vivid.Message) *replacedEnvelop {
	return &replacedEnvelop{
		envelop: envelop,
		message: message,
	}
}

type replacedEnvelop struct {
	envelop vivid.Envelop
	message vivid.Message
}

func (e *replacedEnvelop) System() bool {
	return e.envelop.System()
}
func (e *replacedEnvelop) Sender() vivid.ActorRef {
	return e.envelop.Sender()
}

func (e *replacedEnvelop) Message() vivid.Message {
	return e.message
}

func (e *replacedEnvelop) Receiver() vivid.ActorRef {
	return e.envelop.Receiver()
}
