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

func NewEnvelopWithAsk(agent vivid.ActorRef, sender vivid.ActorRef, message vivid.Message) *Envelop {
	return &Envelop{
		agent:   agent,
		sender:  sender,
		message: message,
	}
}

type Envelop struct {
	agent   vivid.ActorRef // Future 的情况下为被代理的 ActorRef
	sender  vivid.ActorRef // 消息的发送者 ActorRef
	message vivid.Message  // 消息
}

func (e *Envelop) Sender() vivid.ActorRef {
	return e.sender
}

func (e *Envelop) Message() vivid.Message {
	return e.message
}

func (e *Envelop) Agent() vivid.ActorRef {
	return e.agent
}
