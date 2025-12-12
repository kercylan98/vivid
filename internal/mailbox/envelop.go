package mailbox

import (
	"github.com/kercylan98/vivid"
)

var (
	_ vivid.Envelop = (*Envelop)(nil)
)

var EnvelopProvider = vivid.EnvelopProviderFN(func(system bool, agent, sender vivid.ActorRef, message any) vivid.Envelop {
	return NewEnvelop(system, agent, sender, message)
})

func NewEnvelop(system bool, agent, sender vivid.ActorRef, message vivid.Message) *Envelop {
	return &Envelop{
		system:  system,
		agent:   agent,
		sender:  sender,
		message: message,
	}
}

func NewEnvelopWithTell(system bool, sender vivid.ActorRef, message vivid.Message) *Envelop {
	return &Envelop{
		system:  system,
		message: message,
		sender:  sender,
	}
}

func NewEnvelopWithAsk(system bool, agent vivid.ActorRef, sender vivid.ActorRef, message vivid.Message) *Envelop {
	return &Envelop{
		system:  system,
		agent:   agent,
		sender:  sender,
		message: message,
	}
}

type Envelop struct {
	system  bool           // 是否为系统消息
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

func (e *Envelop) System() bool {
	return e.system
}
