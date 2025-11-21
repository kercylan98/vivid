package mailbox

import (
	"sync"

	"github.com/kercylan98/vivid"
)

var (
	_ vivid.Mailbox = &UnboundedMailbox{}
)

func NewUnboundedMailbox() *UnboundedMailbox {
	return &UnboundedMailbox{}
}

type UnboundedMailbox struct {
	// 先简单实现一个无界的队列
	mu    sync.Mutex
	queue []vivid.Envelop
}

func (m *UnboundedMailbox) Enqueue(envelop vivid.Envelop) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = append(m.queue, envelop)
}

func (m *UnboundedMailbox) Dequeue() vivid.Envelop {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.queue) == 0 {
		return nil
	}
	envelop := m.queue[0]
	m.queue = m.queue[1:]
	return envelop
}
