package actor

import (
	"reflect"
	"sync/atomic"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

func newKilledHandler(ctx *Context, message *vivid.OnKilled, behavior vivid.Behavior) *killedHandler {
	return &killedHandler{
		ctx:      ctx,
		message:  message,
		behavior: behavior,
	}
}

type killedHandler struct {
	ctx              *Context
	message          *vivid.OnKilled
	behavior         vivid.Behavior
	selfKilledMessage *vivid.OnKilled
	restarting       bool
	shouldContinue   bool
}

// handleChildDeath 处理子 Actor 死亡
func (h *killedHandler) handleChildDeath() error {
	if !h.message.Ref.Equals(h.ctx.ref) {
		delete(h.ctx.children, h.message.Ref.GetPath())
		h.behavior(h.ctx)
	}
	return nil
}

// checkAndMarkKilled 检查并标记为 killed
func (h *killedHandler) checkAndMarkKilled() error {
	// 如果还有子 Actor，则不处理自身死亡
	if len(h.ctx.children) != 0 || !atomic.CompareAndSwapInt32(&h.ctx.state, killing, killed) {
		h.shouldContinue = false
		return nil
	}
	h.shouldContinue = true
	return nil
}

// prepareSelfKilledMessage 准备自身死亡消息
func (h *killedHandler) prepareSelfKilledMessage() error {
	if !h.shouldContinue {
		return nil
	}
	h.selfKilledMessage = &vivid.OnKilled{Ref: h.ctx.ref}
	h.ctx.envelop = mailbox.NewEnvelop(true, h.ctx.Sender(), h.ctx.ref, h.selfKilledMessage)
	h.restarting = h.ctx.restarting != nil
	return nil
}

// executeBehavior 执行 behavior
func (h *killedHandler) executeBehavior() error {
	if !h.shouldContinue {
		return nil
	}

	if h.restarting {
		// 失败意味着资源可能无法正确释放，但不应阻止新实例的创建。
		// 可能存在资源泄漏，应当记录警告
		logger := h.ctx.Logger()
		if !h.ctx.restarting.recoverExec(logger, "on killed", false, func() error {
			h.behavior(h.ctx)
			return nil
		}) {
			logger.Warn("restart killed failed; resources may not have been properly released",
				log.String("path", h.ctx.ref.GetPath()),
				log.String("reason", h.ctx.restarting.Reason),
				log.Any("fault", h.ctx.restarting.Fault),
				log.String("stack", string(h.ctx.restarting.Stack)))
		}
	} else {
		h.behavior(h.ctx)
	}
	return nil
}

// cleanupIfNotRestarting 如果不是重启状态，进行清理
func (h *killedHandler) cleanupIfNotRestarting() error {
	if !h.shouldContinue || h.restarting {
		return nil
	}

	h.ctx.EventStream().UnsubscribeAll(h.ctx)
	h.ctx.system.removeActorContext(h.ctx)

	// 通知所有监听者
	for _, watcher := range h.ctx.watchers {
		h.ctx.tell(true, watcher, h.selfKilledMessage)
	}

	// 通知父节点
	if h.ctx.parent != nil {
		h.ctx.tell(true, h.ctx.parent, h.selfKilledMessage)
	}

	// 通知事件流
	h.ctx.EventStream().Publish(h.ctx, ves.ActorKilledEvent{
		ActorRef: h.ctx.ref,
		Type:     reflect.TypeOf(h.ctx.actor),
	})
	return nil
}

// cleanupScheduler 清理调度器
func (h *killedHandler) cleanupScheduler() error {
	if !h.shouldContinue {
		return nil
	}
	// 清理调度器，重启也清理
	h.ctx.scheduler.Clear()
	h.ctx.Logger().Debug("actor killed",
		log.String("path", h.ctx.ref.GetPath()),
		log.Bool("restarting", h.restarting))
	return nil
}

// handleRestart 如果是重启状态，处理重启逻辑
func (h *killedHandler) handleRestart() error {
	if !h.shouldContinue || !h.restarting {
		return nil
	}

	// 如果提供了提供者，则使用提供者提供新的 Actor 实例
	if h.ctx.options.Provider != nil {
		h.ctx.actor = h.ctx.options.Provider.Provide()
	}
	h.ctx.behaviorStack.Clear().Push(h.ctx.actor.OnReceive)
	h.ctx.Logger().Debug("actor restarted", log.String("path", h.ctx.ref.GetPath()))

	// 触发重启后的回调
	var success = true
	if restartedActor, ok := h.ctx.actor.(vivid.RestartedActor); ok {
		success = h.ctx.restarting.recoverExec(h.ctx.Logger(), "on restarted", true, func() error {
			return restartedActor.OnRestarted(h.ctx)
		})
	}

	// 触发生命周期
	if preLaunchActor, ok := h.ctx.actor.(vivid.PrelaunchActor); ok && success {
		success = h.ctx.restarting.recoverExec(h.ctx.Logger(), "on pre launch", true, func() error {
			return preLaunchActor.OnPrelaunch(h.ctx)
		})
	}

	// 当子 Actor 重启失败时，不再通知父 Actor 其死亡，而是让其进入"僵尸状态"，避免异常状态扩散。
	if !success {
		// 记录错误并释放资源
		h.ctx.Logger().Error("restart failed; actor is now in zombie state", log.String("path", h.ctx.ref.GetPath()))
		h.ctx.system.removeActorContext(h.ctx)

		// 现有的 ActorRef 缓存中可能持有该邮箱，应当快速排空且进入死信息，避免内存长时间驻留
		h.ctx.mailbox.Resume()
	} else {
		h.ctx.restarting = nil
		atomic.StoreInt32(&h.ctx.state, running)
		h.ctx.tell(true, h.ctx.parent, new(vivid.OnLaunch))
		h.ctx.mailbox.Resume()

		// 通知事件流
		eventStream := h.ctx.EventStream()
		eventStream.Publish(h.ctx, ves.ActorRestartedEvent{
			ActorRef: h.ctx.ref,
			Type:     reflect.TypeOf(h.ctx.actor),
		})
		eventStream.Publish(h.ctx, ves.ActorMailboxResumedEvent{
			ActorRef: h.ctx.ref,
			Type:     reflect.TypeOf(h.ctx.actor),
		})
	}
	return nil
}
