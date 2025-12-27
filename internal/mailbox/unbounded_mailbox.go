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
	buffer       *queues.RingQueue    // 普通消息队列
	systemBuffer *queues.RingQueue    // 系统消息队列
	handler      vivid.EnvelopHandler // 消息处理器
	status       uint32               // 状态
	paused       uint32               // 是否暂停普通消息处理
	num          int32                // 用户消息数量
	systemNum    int32                // 系统消息数量
}

func (m *UnboundedMailbox) Pause() {
	atomic.StoreUint32(&m.paused, 1)
}

func (m *UnboundedMailbox) Resume() {
	if atomic.CompareAndSwapUint32(&m.paused, 1, 0) {
		if atomic.CompareAndSwapUint32(&m.status, idle, processing) {
			go m.process()
		}
	}
}

func (m *UnboundedMailbox) IsPaused() bool {
	return atomic.LoadUint32(&m.paused) == 1
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
		// 优先处理系统消息
		for {
			if msg, ok = m.systemBuffer.Pop(); ok {
				atomic.AddInt32(&m.systemNum, -1)
				m.handler.HandleEnvelop(msg.(vivid.Envelop))
			} else {
				break
			}
		}

		// 检查邮箱是否暂停，暂停时忽略普通消息处理
		if atomic.LoadUint32(&m.paused) == 1 {
			return
		}

		// 处理普通消息
		if msg, ok = m.buffer.Pop(); ok {
			atomic.AddInt32(&m.num, -1)
			m.handler.HandleEnvelop(msg.(vivid.Envelop))
		} else {
			return
		}
	}
}
