package vivid

import (
	"github.com/kercylan98/vivid/src/internal/queues"
	"sync/atomic"
	"unsafe"
)

const (
	mailboxStatusIdle uint32 = iota
	mailboxStatusRunning
)

var (
	_ Mailbox = (*defaultMailbox)(nil)
)

func defaultMailboxProvider() Mailbox {
	return &defaultMailbox{}
}

type Recipient interface {
	// OnReceiveEnvelope 处理收到的信封
	OnReceiveEnvelope(envelope *Envelope)
}

type Mailbox interface {
	// Init 初始化邮箱
	Init(recipient Recipient, dispatcher Dispatcher)

	// Suspend 并发安全的暂停邮箱
	Suspend()

	// Resume 并发安全的恢复邮箱
	Resume()

	// Delivery 投递消息到邮箱
	Delivery(envelope *Envelope)
}

type MailboxProvider interface {
	Provide() Mailbox
}

type MailboxProviderFn func() Mailbox

func (fn MailboxProviderFn) Provide() Mailbox {
	return fn()
}

type defaultMailbox struct {
	dispatcher  Dispatcher
	recipient   Recipient
	queue       *queues.LFQueue
	systemQueue *queues.LFQueue
	status      uint32
	sysNum      int32
	userNum     int32
	suspended   uint32
}

func (m *defaultMailbox) Init(recipient Recipient, dispatcher Dispatcher) {
	m.recipient = recipient
	m.dispatcher = dispatcher
	m.queue = queues.NewLFQueue()
	m.systemQueue = queues.NewLFQueue()
}

func (m *defaultMailbox) Suspend() {
	atomic.StoreUint32(&m.suspended, 1)
}

func (m *defaultMailbox) Resume() {
	atomic.StoreUint32(&m.suspended, 0)
	m.dispatch()
}

func (m *defaultMailbox) Delivery(envelope *Envelope) {
	switch envelope.MessageType {
	case UserMessage:
		m.queue.Push(unsafe.Pointer(envelope))
		atomic.AddInt32(&m.userNum, 1)
		m.dispatch()
	case SystemMessage:
		m.systemQueue.Push(unsafe.Pointer(envelope))
		atomic.AddInt32(&m.sysNum, 1)
		m.dispatch()
	default:
		panic("unknown message type")
	}
}

func (m *defaultMailbox) dispatch() {
	if atomic.CompareAndSwapUint32(&m.status, mailboxStatusIdle, mailboxStatusRunning) {
		m.dispatcher.Dispatch(m.process)
	}
}

func (m *defaultMailbox) process() {
	for {
		m.processHandle()
		atomic.StoreUint32(&m.status, mailboxStatusIdle)
		notEmpty := atomic.LoadInt32(&m.sysNum) > 0 || (atomic.LoadUint32(&m.suspended) == 0 && atomic.LoadInt32(&m.userNum) > 0)
		if !notEmpty {
			break
		} else if !atomic.CompareAndSwapUint32(&m.status, mailboxStatusIdle, mailboxStatusRunning) {
			break
		}
	}
}

func (m *defaultMailbox) processHandle() {
	var envelope *Envelope
	for {
		if ptr := m.systemQueue.Pop(); ptr != nil {
			envelope = (*Envelope)(ptr)
			atomic.AddInt32(&m.sysNum, -1)
			m.recipient.OnReceiveEnvelope(envelope)
			continue
		}

		if atomic.LoadUint32(&m.suspended) == 1 {
			return
		}

		if ptr := m.queue.Pop(); ptr != nil {
			envelope = (*Envelope)(ptr)
			atomic.AddInt32(&m.userNum, -1)
			m.recipient.OnReceiveEnvelope(envelope)
			continue
		}
		break
	}
}
