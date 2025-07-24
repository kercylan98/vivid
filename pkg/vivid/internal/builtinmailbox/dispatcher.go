package builtinmailbox

import (
	"github.com/kercylan98/vivid/pkg/vivid/mailbox"
	"sync/atomic"
)

var _ mailbox.Dispatcher = (*Dispatcher)(nil)

func NewDispatcher(executor mailbox.Executor) *Dispatcher {
	return &Dispatcher{
		executor: executor,
	}
}

type Dispatcher struct {
	status   uint32
	executor mailbox.Executor
}

func (d *Dispatcher) Dispatch(mailbox mailbox.Mailbox) {
	// 无论 CAS 是否成功，都要确保在状态变为 idle 后再次检查是否有新消息
	if atomic.CompareAndSwapUint32(&d.status, 0, 1) {
		go d.dispatch(mailbox)
	}
}

func (d *Dispatcher) dispatch(m mailbox.Mailbox) {
	// 持续处理直到没有消息为止
	for {
		processed := d.process(m)

		// 如果这一轮没有处理任何消息，尝试退出
		if !processed {
			// 设置状态为 idle
			atomic.StoreUint32(&d.status, 0)

			// 再次检查是否有新消息到达（避免在设置 idle 前有新消息入队的竞态条件）
			if (m.GetSystemMessageNum() > 0) || (!m.Suspended() && m.GetUserMessageNum() > 0) {
				// 如果有新消息，尝试重新获取运行状态
				// 如果 CAS 失败，说明其他 goroutine 已经在处理了，可以安全退出
				if atomic.CompareAndSwapUint32(&d.status, 0, 1) {
					continue // 继续处理
				}
			}
			break
		}
	}
}

func (d *Dispatcher) process(m mailbox.Mailbox) (processed bool) {
	var message any

	// 处理系统消息
	for {
		if message = m.PopSystemMessage(); message != nil {
			d.executor.OnSystemMessage(message)
			processed = true
			continue
		}
		break
	}

	// 如果被挂起，直接返回
	if m.Suspended() {
		return processed
	}

	// 处理用户消息
	for {
		if message = m.PopUserMessage(); message != nil {
			d.executor.OnUserMessage(message)
			processed = true
			continue
		}
		break
	}

	return processed
}
