package mailbox

import (
	"github.com/kercylan98/vivid"
)

var (
	_ vivid.Envelop = (*Envelop)(nil)
)

func NewEnvelop(system bool, sender, receiver vivid.ActorRef, message vivid.Message) *Envelop {
	return &Envelop{
		system:   system,
		sender:   sender,
		message:  message,
		receiver: receiver,
	}
}

type Envelop struct {
	system   bool           // 是否为系统消息
	agent    vivid.ActorRef // Future 的情况下为被代理的 ActorRef, 否则应为空
	sender   vivid.ActorRef // 消息的发送者 ActorRef
	message  vivid.Message  // 消息
	receiver vivid.ActorRef // 接收人（仅限远程）
}

// WithAgent 设置被代理的 ActorRef
func (e *Envelop) WithAgent(agent vivid.ActorRef) *Envelop {
	e.agent = agent
	return e
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

func (e *Envelop) Receiver() vivid.ActorRef {
	return e.receiver
}
