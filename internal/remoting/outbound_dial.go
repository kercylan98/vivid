package remoting

import (
	"container/list"

	"github.com/kercylan98/vivid"
)

// outboundDial 先预留执行名额，再通知端点启动任务；等待期间不占用 goroutine。
// 所有状态由管理器锁保护，执行中的任务只能在退出后释放名额。
type outboundDial struct {
	manager *EndpointManager
	notify  func(*outboundDial)
	wait    *list.Element
	started bool
	done    bool
}

func (e *EndpointManager) requestDial(notify func(*outboundDial)) (*outboundDial, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return nil, false, vivid.ErrorRemotingStopped
	}
	ready := e.dialCount < e.dialLimit
	if !ready && e.waitDials.Len() >= e.waitLimit {
		return nil, false, vivid.ErrorRemotingDialQueueFull
	}
	dial := &outboundDial{manager: e}
	e.dials[dial] = struct{}{}
	if ready {
		e.dialCount++
	} else {
		dial.notify = notify
		dial.wait = e.waitDials.PushBack(dial)
	}
	return dial, ready, nil
}

func (d *outboundDial) begin() bool {
	d.manager.mu.Lock()
	defer d.manager.mu.Unlock()
	if d.done || d.started || d.wait != nil || d.manager.stopped {
		return false
	}
	d.started = true
	return true
}

func (d *outboundDial) cancel() {
	d.finish(false)
}

func (d *outboundDial) complete() {
	d.finish(true)
}

func (d *outboundDial) finish(completed bool) {
	e := d.manager
	e.mu.Lock()
	if d.done || (d.started && !completed) {
		e.mu.Unlock()
		return
	}
	d.removeLocked()
	var next *outboundDial
	var notify func(*outboundDial)
	if !e.stopped && e.dialCount < e.dialLimit && e.waitDials.Len() > 0 {
		next = e.waitDials.Front().Value.(*outboundDial)
		e.waitDials.Remove(next.wait)
		next.wait = nil
		notify, next.notify = next.notify, nil
		e.dialCount++
	}
	e.mu.Unlock()
	// Actor 投递可能重新进入管理器，通知必须在锁外执行。
	if notify != nil {
		notify(next)
	}
}

func (d *outboundDial) removeLocked() {
	e := d.manager
	if d.wait != nil {
		e.waitDials.Remove(d.wait)
		d.wait = nil
	} else {
		e.dialCount--
	}
	d.done = true
	d.notify = nil
	delete(e.dials, d)
}
