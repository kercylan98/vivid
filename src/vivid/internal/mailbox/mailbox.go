package mailbox

import (
	"sync/atomic"

	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"
	"github.com/kercylan98/vivid/src/vivid/internal/queues"
)

const (
	idle uint32 = iota
	running
)

func NewMailbox() mailbox.Mailbox {
	return newMailbox()
}

func newMailbox() *mailboxImpl {
	return &mailboxImpl{
		queue:       queues.NewRingBuffer(1024),
		systemQueue: queues.NewRingBuffer(32),
	}
}

type mailboxImpl struct {
	dispatcher  mailbox.Dispatcher
	handler     mailbox.Handler
	queue       *queues.RingBuffer
	systemQueue *queues.RingBuffer
	status      uint32
	sysNum      int32
	userNum     int32
	suspended   uint32
}

func (m *mailboxImpl) Initialize(dispatcher mailbox.Dispatcher, handler mailbox.Handler) {
	m.dispatcher = dispatcher
	m.handler = handler
}

func (m *mailboxImpl) Suspend() {
	atomic.StoreUint32(&m.suspended, 1)
}

func (m *mailboxImpl) Resume() {
	atomic.StoreUint32(&m.suspended, 0)
	m.dispatch()
}

func (m *mailboxImpl) HandleSystemMessage(message core.Message) {
	m.systemQueue.Push(message)
	atomic.AddInt32(&m.sysNum, 1)
	m.dispatch()
}

func (m *mailboxImpl) HandleUserMessage(message core.Message) {
	m.queue.Push(message)
	atomic.AddInt32(&m.userNum, 1)
	m.dispatch()
}

func (m *mailboxImpl) dispatch() {
	if atomic.CompareAndSwapUint32(&m.status, idle, running) {
		m.dispatcher.Dispatch(m.process)
	}
}

func (m *mailboxImpl) process() {
	m.processHandle()
	for atomic.LoadInt32(&m.sysNum) > 0 || (atomic.LoadUint32(&m.suspended) == 0 && atomic.LoadInt32(&m.userNum) > 0) {
		m.processHandle()
	}
	atomic.StoreUint32(&m.status, idle)
}

func (m *mailboxImpl) processHandle() {
	var message core.Message

	// 处理系统消息
	for {
		if message = m.systemQueue.Pop(); message != nil {
			atomic.AddInt32(&m.sysNum, -1)
			m.handler.HandleSystemMessage(message)
			continue
		}
		break
	}

	// 如果被挂起，直接返回
	if atomic.LoadUint32(&m.suspended) == 1 {
		return
	}

	// 处理用户消息
	for {
		if message = m.queue.Pop(); message != nil {
			atomic.AddInt32(&m.userNum, -1)
			m.handler.HandleUserMessage(message)
			continue
		}
		break
	}
}
