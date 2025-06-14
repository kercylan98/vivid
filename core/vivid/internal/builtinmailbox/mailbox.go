package builtinmailbox

import (
	mailbox2 "github.com/kercylan98/vivid/core/vivid/mailbox"
	"github.com/kercylan98/vivid/src/queues"
	"sync/atomic"
)

var _ mailbox2.Mailbox = (*Mailbox)(nil)

func NewMailbox(system, user queues.Queue, dispatcher mailbox2.Dispatcher) mailbox2.Mailbox {
	return &Mailbox{
		systemQueue: system,
		userQueue:   user,
		dispatcher:  dispatcher,
	}
}

type Mailbox struct {
	systemQueue queues.Queue
	userQueue   queues.Queue
	userNum     int32
	systemNum   int32
	dispatcher  mailbox2.Dispatcher
	suspend     uint32
}

func (m *Mailbox) GetSystemMessageNum() int32 {
	return atomic.LoadInt32(&m.systemNum)
}

func (m *Mailbox) GetUserMessageNum() int32 {
	return atomic.LoadInt32(&m.userNum)
}

func (m *Mailbox) Suspended() bool {
	return atomic.LoadUint32(&m.suspend) == 1
}

func (m *Mailbox) Suspend() {
	atomic.StoreUint32(&m.suspend, 1)
}

func (m *Mailbox) Resume() {
	atomic.StoreUint32(&m.suspend, 0)
	m.dispatcher.Dispatch(m)
}

func (m *Mailbox) PushSystemMessage(message any) {
	m.systemQueue.Push(message)
	atomic.AddInt32(&m.systemNum, 1)
	m.dispatcher.Dispatch(m)
}

func (m *Mailbox) PushUserMessage(message any) {
	m.userQueue.Push(message)
	atomic.AddInt32(&m.userNum, 1)
	m.dispatcher.Dispatch(m)
}

func (m *Mailbox) PopSystemMessage() (message any) {
	if message = m.systemQueue.Pop(); message != nil {
		atomic.AddInt32(&m.systemNum, -1)
	}
	return
}

func (m *Mailbox) PopUserMessage() (message any) {
	if message = m.userQueue.Pop(); message != nil {
		atomic.AddInt32(&m.userNum, -1)
	}
	return
}
