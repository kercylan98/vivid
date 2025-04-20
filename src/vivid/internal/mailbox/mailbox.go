package mailbox

import (
    "github.com/kercylan98/vivid/src/vivid/internal/core"
    "github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"
    queues2 "github.com/kercylan98/vivid/src/vivid/internal/queues"
    "sync/atomic"
    "unsafe"
)

const (
    mailboxStatusIdle uint32 = iota
    mailboxStatusRunning
)

func NewMailbox() mailbox.Mailbox {
    return &mailboxImpl{
        queue:       queues2.NewRingBuffer(1024),
        systemQueue: queues2.NewLFQueue(),
    }
}

type mailboxImpl struct {
    dispatcher  mailbox.Dispatcher
    handler     mailbox.Handler
    queue       *queues2.RingBuffer
    systemQueue *queues2.LFQueue
    status      uint32
    sysNum      int32
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
    m.systemQueue.Push(unsafe.Pointer(&message))
    atomic.AddInt32(&m.sysNum, 1)
    m.dispatch()
}

func (m *mailboxImpl) HandleUserMessage(message core.Message) {
    m.queue.Push(message)
    m.dispatch()
}

func (m *mailboxImpl) dispatch() {
    if atomic.CompareAndSwapUint32(&m.status, mailboxStatusIdle, mailboxStatusRunning) {
        m.dispatcher.Dispatch(m.process)
    }
}

func (m *mailboxImpl) process() {
    for {
        m.processHandle()
        atomic.StoreUint32(&m.status, mailboxStatusIdle)
        notEmpty := atomic.LoadInt32(&m.sysNum) > 0 || (atomic.LoadUint32(&m.suspended) == 0 && m.queue.Length() > 0)
        if !notEmpty {
            break
        } else if !atomic.CompareAndSwapUint32(&m.status, mailboxStatusIdle, mailboxStatusRunning) {
            break
        }
    }
}

func (m *mailboxImpl) processHandle() {
    var message core.Message
    for {
        if ptr := m.systemQueue.Pop(); ptr != nil {
            message = *(*core.Message)(ptr)
            atomic.AddInt32(&m.sysNum, -1)
            m.handler.HandleSystemMessage(message)
            continue
        }

        if atomic.LoadUint32(&m.suspended) == 1 {
            return
        }

        if message = m.queue.Pop(); message != nil {
            m.handler.HandleUserMessage(message)
            continue
        }
        break
    }
}
