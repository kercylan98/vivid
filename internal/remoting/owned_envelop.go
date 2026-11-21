package remoting

import (
	"sync"

	"github.com/kercylan98/vivid"
)

// NewOwnedEnvelop 将一次远程发送持有的资源交给 Remoting，不改变线上消息格式。
// 仅向远程 Mailbox.Enqueue 投递一次，之后不能修改负载或自行释放资源。
// complete 在成功写出或最终放弃、且所有编码和写出操作结束后调用一次。
// written 只表示本地发送完成，不代表对端接收、持久化或业务处理；回调须快速返回。
// 回调可来自 writer 或关闭调用方，串行状态须通过消息交回所属 Actor。
// 完成后原始消息仍可能被死信持有，释放额度或外部句柄时不能覆盖其不可变负载。
func NewOwnedEnvelop(envelop vivid.Envelop, complete func(written bool)) vivid.Envelop {
	return &ownedEnvelop{Envelop: envelop, complete: complete}
}

type ownedEnvelop struct {
	vivid.Envelop
	mu       sync.Mutex
	active   int
	done     bool
	written  bool
	complete func(bool)
}

func (e *ownedEnvelop) retain() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.done {
		return false
	}
	e.active++
	return true
}

func (e *ownedEnvelop) release() {
	e.mu.Lock()
	e.active--
	complete, written := e.takeCompletion()
	e.mu.Unlock()
	if complete != nil {
		complete(written)
	}
}

func (e *ownedEnvelop) finish(written bool) bool {
	e.mu.Lock()
	if e.done {
		e.mu.Unlock()
		return false
	}
	e.done, e.written = true, written
	complete, result := e.takeCompletion()
	e.mu.Unlock()
	if complete != nil {
		complete(result)
	}
	return true
}

// takeCompletion 只在持锁时调用，实际回调始终在锁外执行。
func (e *ownedEnvelop) takeCompletion() (func(bool), bool) {
	if !e.done || e.active != 0 {
		return nil, false
	}
	complete := e.complete
	e.complete = nil
	return complete, e.written
}

func ownedOutboundEnvelop(envelop vivid.Envelop) *ownedEnvelop {
	if pending, ok := envelop.(*outboundEnvelop); ok {
		envelop = pending.Envelop
	}
	owned, _ := envelop.(*ownedEnvelop)
	return owned
}

func beginOutboundWrite(envelop vivid.Envelop) bool {
	if !outboundEnvelopActive(envelop) {
		return false
	}
	if owned := ownedOutboundEnvelop(envelop); owned != nil {
		return owned.retain()
	}
	return true
}

func endOutboundWrite(envelop vivid.Envelop) {
	if owned := ownedOutboundEnvelop(envelop); owned != nil {
		owned.release()
	}
}
