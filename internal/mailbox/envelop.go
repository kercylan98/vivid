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
	sender   vivid.ActorRef // 消息的发送者 ActorRef
	receiver vivid.ActorRef // 接收人（仅限远程）
	message  vivid.Message  // 消息
	system   bool           // 是否为系统消息
}

func (e *Envelop) Sender() vivid.ActorRef {
	return e.sender
}

func (e *Envelop) Message() vivid.Message {
	return e.message
}

func (e *Envelop) System() bool {
	return e.system
}

func (e *Envelop) Receiver() vivid.ActorRef {
	return e.receiver
}
