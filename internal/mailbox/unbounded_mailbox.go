package mailbox

import (
	"sync"
	"sync/atomic"

	"github.com/kercylan98/vivid"
)

var (
	_ vivid.Mailbox = &UnboundedMailbox{}
)

func NewUnboundedMailbox(initialSize int64, handler vivid.EnvelopHandler) *UnboundedMailbox {
	return &UnboundedMailbox{
		buffer:       make([]vivid.Envelop, 0, initialSize),
		systemBuffer: make([]vivid.Envelop, 0, initialSize),
		handler:      handler,
	}
}

type UnboundedMailbox struct {
	buffer       []vivid.Envelop      // 普通消息队列
	systemBuffer []vivid.Envelop      // 系统消息队列
	envelops     any                  // 当前信封分配块，已投递的信封不复用
	envelopIndex int                  // 当前信封块的分配位置
	lock         sync.Mutex           // 保护消息入队及消费者调度
	status       uint32               // 状态，由 lock 保护
	_            [64]byte             // 隔离入队与消费状态，减少缓存行争用
	batch        []vivid.Envelop      // 当前消费的普通消息批次，仅由消费者访问
	batchIndex   int                  // 当前批次的消费位置
	handler      vivid.EnvelopHandler // 消息处理器
	paused       uint32               // 是否暂停普通消息处理
	systemNum    int32                // 系统消息数量
}

func (m *UnboundedMailbox) Pause() {
	atomic.StoreUint32(&m.paused, 1)
}

func (m *UnboundedMailbox) Resume() {
	m.lock.Lock()
	atomic.StoreUint32(&m.paused, 0)
	start := m.status == idle
	if start {
		m.status = processing
	}
	m.lock.Unlock()
	if start {
		go m.process()
	}
}

func (m *UnboundedMailbox) IsPaused() bool {
	return atomic.LoadUint32(&m.paused) == 1
}

func (m *UnboundedMailbox) Enqueue(envelop vivid.Envelop) {
	system := envelop.System()
	m.lock.Lock()
	start := m.enqueueLocked(envelop, system)
	m.lock.Unlock()
	if start {
		go m.process()
	}
}

func (m *UnboundedMailbox) enqueueLocked(envelop vivid.Envelop, system bool) bool {
	if system {
		m.systemBuffer = append(m.systemBuffer, envelop)
		atomic.AddInt32(&m.systemNum, 1)
	} else {
		m.buffer = append(m.buffer, envelop)
	}

	start := m.status == idle && (system || !m.IsPaused())
	if start {
		m.status = processing
	}
	return start
}

func (m *UnboundedMailbox) process() {
	for {
		// 优先处理系统消息
		if atomic.LoadInt32(&m.systemNum) > 0 {
			m.lock.Lock()
			systemBatch := m.systemBuffer
			m.systemBuffer = nil
			atomic.StoreInt32(&m.systemNum, 0)
			m.lock.Unlock()
			for _, envelop := range systemBatch {
				m.handler.HandleEnvelop(envelop)
			}
			continue
		}

		// 批次内消费无需与生产者争用队列锁，暂停时保留尚未处理的消息
		if !m.IsPaused() && m.batchIndex < len(m.batch) {
			envelop := m.batch[m.batchIndex]
			m.batch[m.batchIndex] = nil
			m.batchIndex++
			m.handler.HandleEnvelop(envelop)
			continue
		}

		m.lock.Lock()
		// 与入队、恢复在同一把锁下交接状态，避免退出消费者时遗漏唤醒
		if len(m.systemBuffer) > 0 {
			m.lock.Unlock()
			continue
		}
		if !m.IsPaused() {
			if m.batchIndex < len(m.batch) {
				m.lock.Unlock()
				continue
			}
			if len(m.buffer) > 0 {
				m.buffer, m.batch = m.batch[:0], m.buffer
				m.batchIndex = 0
				m.lock.Unlock()
				continue
			}
		}
		m.status = idle
		m.lock.Unlock()
		return
	}
}
