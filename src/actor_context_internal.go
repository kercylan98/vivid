package vivid

import (
	"log/slog"
	"sync/atomic"
	"time"
)

const (
	actorStatusAlive       uint32 = iota // Actor 存活状态
	actorStatusRestarting                // Actor 正在重启
	actorStatusTerminating               // Actor 正在终止
	actorStatusTerminated                // Actor 已终止
)

var (
	_ Recipient = (*internalActorContext)(nil) // 确保 internalActorContext 实现了 Recipient 接口
	_ Process   = (*internalActorContext)(nil) // 确保 internalActorContext 实现了 Process 接口
)

type internalActorContext struct {
	*actorContext
	ref           ActorRef                    // Actor 引用
	mailbox       Mailbox                     // Actor 的邮箱
	terminated    atomic.Bool                 // Actor 是否已终止
	status        atomic.Uint32               // Actor 状态
	envelope      Envelope                    // 当前消息
	watchHandlers map[ActorRef][]WatchHandler // 监视处理器（当 Key 存在表示正在监视目标）
	watchers      map[ActorRef]struct{}       // 该 Actor 的监视者
}

func (ctx *internalActorContext) init(actorContext *actorContext, mailbox Mailbox) {
	ctx.actorContext = actorContext
	ctx.mailbox = mailbox

	ctx.Logger().Debug("spawn", slog.String("actor", ctx.ref.String()))

	var launchContext map[any]any
	if ctx.config.FetchLaunchContextProvider() != nil {
		launchContext = ctx.config.FetchLaunchContextProvider().Provide()
	}

	ctx.Tell(ctx.Ref(), newOnLaunch(time.Now(), launchContext))
}

func (ctx *internalActorContext) GetID() ID {
	return ctx.ref
}

// Send 该函数为 Process 接口的实现，用于将消息交由邮箱处理
//   - 内部不建议甚至请拒绝直接调用该函数，除非你明确知道你在做什么
//   - 如果是对于自身的消息且无需考虑优先级的消息，可直接调用 actorContext.onProcessMessage 函数来得到更高效的处理
//   - 如果是对于其他 Actor 的消息，应该调用 actorContext.sendToProcess 函数来发送消息
func (ctx *internalActorContext) Send(envelope Envelope) {
	switch envelope.GetMessage().(type) {
	case *onResumeMailboxMessage:
		ctx.mailbox.Resume()
	case *onSuspendMailboxMessage:
		ctx.mailbox.Suspend()
	default:
		ctx.mailbox.Delivery(envelope)
	}
}

func (ctx *internalActorContext) Terminated() bool {
	return ctx.terminated.Load()
}

func (ctx *internalActorContext) OnTerminate(operator ID) {
	ctx.terminated.Store(true)
}

func (ctx *internalActorContext) sendToProcess(envelope Envelope) {
	process, daemon := ctx.actorSystem.processManager.getProcess(envelope.GetReceiver())
	if daemon {
		ctx.Logger().Warn("sendToProcess", slog.Any("process not found", envelope))
		return
	}
	process.Send(envelope)
}

func (ctx *internalActorContext) onAccident(reason any) {
	//TODO implement me
	panic("implement me")
}

func (ctx *internalActorContext) OnReceiveEnvelope(envelope Envelope) {
	if ctx.status.Load() >= actorStatusTerminating {
		ctx.Logger().Warn("OnReceiveEnvelope", slog.String("actor is terminating", ctx.ref.GetPath()))

		ctx.Tell(ctx.actorSystem.Ref(), envelope)
		return
	}

	ctx.onProcessMessage(envelope)
}

func (ctx *internalActorContext) onProcessMessage(envelope Envelope) {
	ctx.envelope = envelope
	switch envelope.GetMessageType() {
	case SystemMessage:
		ctx.onProcessSystemMessage(envelope)
	case UserMessage:
		ctx.onProcessUserMessage(envelope)
	default:
		panic("unknown message type")
	}
}

func (ctx *internalActorContext) onProcessSystemMessage(envelope Envelope) {
	switch m := envelope.GetMessage().(type) {
	case OnLaunch:
		ctx.onProcessUserMessage(envelope)
	case OnKill:
		ctx.onKill(envelope, m)
	case OnKilled:
		ctx.onKilled()
	case OnWatch:
		ctx.onWatch()
	case OnUnwatch:
		ctx.onUnwatch()
	case OnPing:
		ctx.Reply(ctx.systemConfig().FetchRemoteMessageBuilder().BuildPong(m))
	default:
		panic("unknown system message")
	}
}

func (ctx *internalActorContext) onProcessUserMessage(envelope Envelope) {
	switch m := envelope.GetMessage().(type) {
	case OnWatchStopped:
		ctx.onWatchStopped(envelope, m)
	case OnKill:
		ctx.onProcessUserMessageWithActor()
		ctx.onKill(envelope, m) // 用户消息已被处理，转为终止 Actor
	default:
		ctx.onProcessUserMessageWithActor()
	}
}

func (ctx *internalActorContext) onProcessUserMessageWithActor() {
	// 交由用户处理的消息需保证异常捕获
	defer func() {
		if reason := recover(); reason != nil {
			ctx.onAccident(reason)
		}
	}()

	ctx.actor.OnReceive(ctx)
}

func (ctx *internalActorContext) onKill(envelope Envelope, event OnKill) {
	// 当 Actor 处于 actorStatusTerminating 状态时，表明 Actor 已经在终止中，此刻也不应该继续接收新的消息
	if !ctx.status.CompareAndSwap(actorStatusAlive, actorStatusTerminating) {
		// 转换状态为终止中，如果失败，表面可能已经终止
		// 重复终止一般是在销毁时再次尝试终止导致，该逻辑可避免非幂等影响
		return
	}

	// 等待用户处理持久化或清理工作
	ctx.onProcessUserMessage(envelope)

	// 终止子 Actor
	ctx.onKillChildren(event)

	// 刷新终止状态
	ctx.refreshTerminateStatus()
}

func (ctx *internalActorContext) onKillChildren(event OnKill) {
	var messageType = SystemMessage
	if event.IsPoison() {
		messageType = UserMessage
	}
	for _, ref := range ctx.children {
		ctx.tell(ref, event, messageType)
	}
}

func (ctx *internalActorContext) refreshTerminateStatus() {
	// 如果子 Actor 未全部终止或已终止，那么停止终止流程
	if len(ctx.children) > 0 && !ctx.status.CompareAndSwap(actorStatusTerminating, actorStatusTerminated) {
		return
	}

	// 通知监视者
	if ctx.watchers != nil {
		onWatchStopped := ctx.systemConfig().FetchRemoteMessageBuilder().BuildOnWatchStopped()
		for watcher := range ctx.watchers {
			ctx.tell(watcher, onWatchStopped, UserMessage)
		}
	}

	// 通知父 Actor
	if ctx.parent != nil {
		onKilled := ctx.systemConfig().FetchRemoteMessageBuilder().BuildOnKilled(ctx.Ref())
		ctx.tell(ctx.parent, onKilled, SystemMessage)
	}
}

func (ctx *internalActorContext) onWatch() {
	if ctx.status.Load() >= actorStatusTerminating {
		onWatchStopped := ctx.systemConfig().FetchRemoteMessageBuilder().BuildOnWatchStopped()
		ctx.tell(ctx.Sender(), onWatchStopped, UserMessage) // 通过用户消息告知已死
		return
	}
	if ctx.watchers == nil {
		ctx.watchers = make(map[ActorRef]struct{})
	}
	ctx.watchers[ctx.Sender()] = struct{}{}
}

func (ctx *internalActorContext) onUnwatch() {
	delete(ctx.watchers, ctx.Sender())
	if len(ctx.watchers) == 0 {
		ctx.watchers = nil
	}
}

func (ctx *internalActorContext) onWatchStopped(e Envelope, m OnWatchStopped) {
	sender := ctx.Sender()
	handlers, exist := ctx.watchHandlers[sender]
	if !exist {
		return // 未监视该 Actor（可能已取消）
	}

	if len(handlers) == 0 {
		// 未设置处理器，交由用户处理
		ctx.onProcessUserMessageWithActor()
	} else {
		// 交由处理器处理
		for _, handler := range handlers {
			handler.Handle(ctx, m)
		}
	}

	// 释放处理器
	delete(ctx.watchHandlers, sender)
	if len(ctx.watchHandlers) == 0 {
		ctx.watchHandlers = nil
	}

}

func (ctx *internalActorContext) onKilled() {
	// 子 Actor 终止，释放资源
	child := ctx.Sender()
	delete(ctx.children, child.GetPath())
	if len(ctx.children) == 0 {
		ctx.children = nil
	}

	ctx.refreshTerminateStatus()
}
