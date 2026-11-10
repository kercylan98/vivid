package mailbox

import "github.com/kercylan98/vivid"

var (
	_ vivid.Envelop = (*messageEnvelop[vivid.ActorRef])(nil)
)

// EnqueueMessage 使用具体引用类型按块分配普通消息信封，已投递的信封不回收复用。
func EnqueueMessage[R vivid.ActorRef](m *UnboundedMailbox, sender, receiver R, message vivid.Message) {
	m.lock.Lock()
	block, ok := m.envelops.(*messageEnvelopBlock[R])
	if !ok || m.envelopIndex == len(block) {
		block = new(messageEnvelopBlock[R])
		m.envelops = block
		m.envelopIndex = 0
	}
	envelop := &block[m.envelopIndex]
	m.envelopIndex++
	*envelop = messageEnvelop[R]{sender: sender, receiver: receiver, message: message}
	start := m.enqueueLocked(envelop, false)
	m.lock.Unlock()
	if start {
		go m.process()
	}
}

// messageEnvelopBlock 在 64 位指针引用场景下占用 512B，同块信封共享 GC 生命周期。
type messageEnvelopBlock[R vivid.ActorRef] [16]messageEnvelop[R]

type messageEnvelop[R vivid.ActorRef] struct {
	sender   R             // 消息的发送者 ActorRef
	receiver R             // 接收人
	message  vivid.Message // 消息
}

func (e *messageEnvelop[R]) Sender() vivid.ActorRef {
	return e.sender
}

func (e *messageEnvelop[R]) Receiver() vivid.ActorRef {
	return e.receiver
}

func (e *messageEnvelop[R]) Message() vivid.Message {
	return e.message
}

func (e *messageEnvelop[R]) System() bool {
	return false
}
