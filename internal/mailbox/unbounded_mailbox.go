package mailbox

import (
	"sync/atomic"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/queues"
)

var (
	_ vivid.Mailbox = &UnboundedMailbox{}
)

func NewUnboundedMailbox(initialSize int64, handler vivid.EnvelopHandler) *UnboundedMailbox {
	return &UnboundedMailbox{
		buffer:       queues.New(initialSize),
		systemBuffer: queues.New(initialSize),
		handler:      handler,
	}
}

type UnboundedMailbox struct {
	num          int32
	status       uint32
	buffer       *queues.RingQueue
	systemNum    int32
	systemBuffer *queues.RingQueue
	handler      vivid.EnvelopHandler
}

func (m *UnboundedMailbox) Enqueue(envelop vivid.Envelop) {
	if envelop.System() {
		m.systemBuffer.Push(envelop)
		atomic.AddInt32(&m.systemNum, 1)
	} else {
		m.buffer.Push(envelop)
		atomic.AddInt32(&m.num, 1)
	}

	if atomic.CompareAndSwapUint32(&m.status, idle, processing) {
		go m.process()
	}
}

func (m *UnboundedMailbox) process() {
process:
	m.processHandle()

	atomic.StoreUint32(&m.status, idle)
	user := atomic.LoadInt32(&m.num)
	system := atomic.LoadInt32(&m.systemNum)
	if user > 0 || system > 0 {
		if atomic.CompareAndSwapUint32(&m.status, idle, processing) {
			goto process
		}
	}
}

func (m *UnboundedMailbox) processHandle() {
	var msg any
	var ok bool

	for {
		// 先处理系统消息
		for {
			if msg, ok = m.systemBuffer.Pop(); ok {
				atomic.AddInt32(&m.systemNum, -1)
				m.handler.HandleEnvelop(msg.(vivid.Envelop))
			} else {
				break
			}
		}

		// 再处理用户消息
		if msg, ok = m.buffer.Pop(); ok {
			atomic.AddInt32(&m.num, -1)
			m.handler.HandleEnvelop(msg.(vivid.Envelop))
		} else {
			return
		}
	}
}
