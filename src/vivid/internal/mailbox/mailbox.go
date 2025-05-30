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
	// 无论 CAS 是否成功，都要确保在状态变为 idle 后再次检查是否有新消息
	if atomic.CompareAndSwapUint32(&m.status, idle, running) {
		m.dispatcher.Dispatch(m.process)
	}
}

func (m *mailboxImpl) process() {
	// 持续处理直到没有消息为止
	for {
		processed := m.processHandle()

		// 如果这一轮没有处理任何消息，尝试退出
		if !processed {
			// 设置状态为 idle
			atomic.StoreUint32(&m.status, idle)

			// 再次检查是否有新消息到达（避免在设置 idle 前有新消息入队的竞态条件）
			if (atomic.LoadInt32(&m.sysNum) > 0) ||
				(atomic.LoadUint32(&m.suspended) == 0 && atomic.LoadInt32(&m.userNum) > 0) {
				// 如果有新消息，尝试重新获取运行状态
				// 如果 CAS 失败，说明其他 goroutine 已经在处理了，可以安全退出
				if atomic.CompareAndSwapUint32(&m.status, idle, running) {
					continue // 继续处理
				}
			}
			break
		}
	}
}

func (m *mailboxImpl) processHandle() bool {
	var message core.Message
	processed := false

	// 处理系统消息
	for {
		if message = m.systemQueue.Pop(); message != nil {
			atomic.AddInt32(&m.sysNum, -1)
			m.handler.HandleSystemMessage(message)
			processed = true
			continue
		}
		break
	}

	// 如果被挂起，直接返回
	if atomic.LoadUint32(&m.suspended) == 1 {
		return processed
	}

	// 处理用户消息
	for {
		if message = m.queue.Pop(); message != nil {
			atomic.AddInt32(&m.userNum, -1)
			m.handler.HandleUserMessage(message)
			processed = true
			continue
		}
		break
	}

	return processed
}
