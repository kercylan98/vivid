package vivid

import (
	"fmt"
	"github.com/kercylan98/go-log/log"
	"sync/atomic"
)

const (
	actorStatusAlive       uint32 = iota // Actor 存活状态
	actorStatusRestarting                // Actor 正在重启
	actorStatusTerminating               // Actor 正在终止
	actorStatusTerminated                // Actor 已终止
)

var (
	_ Recipient = (*actorContextRecipient)(nil)
)

func newActorContextRecipient(ctx ActorContext) *actorContextRecipient {
	return &actorContextRecipient{
		ActorContext: ctx,
	}
}

type actorContextRecipient struct {
	ActorContext
	status atomic.Uint32 // Actor 状态
}

func (ctx *actorContextRecipient) OnReceiveEnvelope(envelope Envelope) {
	if status := ctx.status.Load(); status == actorStatusTerminated {
		switch envelope.GetMessage().(type) {
		case OnWatch, OnWatchStopped:
			// 继续执行
		case OnKill:
			// 当子 Actor 正在终止的过程，还未向父 Actor 发送 OnKilled 消息时
			// 父 Actor 如果被终止或主动终止子 Actor，此刻依旧会向子 Actor 发送 OnKill 消息
			// 此时子 Actor 将会被正常流程终止，因此直接返回
			return
		default:
			ctx.Logger().Warn("OnReceiveEnvelope", log.String("actor is terminated", ctx.Ref().String()), log.Int32("status", status), log.String("sender", envelope.GetSender().String()), log.String("message", fmt.Sprintf("%T", envelope.GetMessage())))

			// 如果该 Actor 不是顶级 Actor，那么将消息传递给顶级 Actor 确保异常被记录
			// 如果已经是顶级 Actor，则说明 ActorSystem 正在关闭，需要丢弃消息
			if parent := ctx.Parent(); parent != nil {
				ctx.Tell(ctx.System().Ref(), envelope)
			}
			return
		}
	}

	ctx.onProcessMessage(envelope)
}

func (ctx *actorContextRecipient) onProcessMessage(envelope Envelope) {
	ctx.setEnvelope(envelope)
	switch envelope.GetMessageType() {
	case SystemMessage:
		ctx.onProcessSystemMessage(envelope)
	case UserMessage:
		ctx.onProcessUserMessage(envelope)
	default:
		panic("unknown message type")
	}
}

func (ctx *actorContextRecipient) onProcessSystemMessage(envelope Envelope) {
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
		ctx.deleteWatcher(ctx.Sender())
	case OnPing:
		ctx.Reply(ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildPong(m))
	default:
		panic("unknown system message")
	}
}

func (ctx *actorContextRecipient) onProcessUserMessage(envelope Envelope) {
	switch m := envelope.GetMessage().(type) {
	case OnWatchStopped:
		ctx.onWatchStopped(m)
	case OnKill:
		ctx.onKill(envelope, m) // 用户消息已被处理，转为终止 Actor
	case TimingTask:
		m.Execute(ctx)
	default:
		ctx.onProcessUserMessageWithActor()
	}
}

func (ctx *actorContextRecipient) onProcessUserMessageWithActor() {
	// 交由用户处理的消息需保证异常捕获
	defer func() {
		if reason := recover(); reason != nil {
			ctx.onAccident(reason)
		}
	}()

	ctx.getActor().OnReceive(ctx)
}

func (ctx *actorContextRecipient) onAccident(reason any) {
	//TODO implement me
	panic("implement me")
}

func (ctx *actorContextRecipient) onKill(envelope Envelope, event OnKill) {
	// 当 Actor 处于 actorStatusTerminating 状态时，表明 Actor 已经在终止中
	if !ctx.status.CompareAndSwap(actorStatusAlive, actorStatusTerminating) {
		// 转换状态为终止中，如果失败，表面可能已经终止
		// 重复终止一般是在销毁时再次尝试终止导致，该逻辑可避免非幂等影响
		return
	}

	// 等待用户处理持久化或清理工作
	ctx.onProcessUserMessageWithActor()

	// 终止子 Actor
	ctx.onKillChildren(event)

	// 刷新终止状态
	ctx.refreshTerminateStatus()
}

func (ctx *actorContextRecipient) onKillChildren(event OnKill) {
	var messageType = SystemMessage
	if event.IsPoison() {
		messageType = UserMessage
	}
	for _, ref := range ctx.getChildren() {
		ctx.tell(ref, event, messageType)
	}
}

func (ctx *actorContextRecipient) refreshTerminateStatus() {
	// 如果子 Actor 未全部终止或已终止，那么停止终止流程
	if len(ctx.getChildren()) > 0 {
		return
	}

	// 此刻开始终止自身
	if !ctx.status.CompareAndSwap(actorStatusTerminating, actorStatusTerminated) {
		return
	}

	// 清除所有已设置的定时器
	// 定时器可能在生命周期中动态设定，避免出现冲突，在死亡和重启时候均需要清除
	ctx.getTimingWheel().Clear()

	// 此刻已经死亡，记录日志
	ctx.onKillLog()

	// 通知监视者
	if watchers := ctx.getWatchers(); watchers != nil {
		onWatchStopped := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnWatchStopped(ctx.Ref())
		for watcher := range watchers {
			// 如果监视者是自己，此刻由于已经终止，将无法通过消息队列发送消息，因此直接调用
			if watcher.Equal(ctx.Ref()) {
				ctx.onWatchStopped(onWatchStopped)
				continue
			}
			ctx.tell(watcher, onWatchStopped, UserMessage)
		}
	}

	// 通知父 Actor
	if parent := ctx.Parent(); parent != nil {
		onKilled := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnKilled(ctx.Ref())
		ctx.tell(parent, onKilled, SystemMessage)
	}
}

func (ctx *actorContextRecipient) onWatch() {
	if ctx.status.Load() == actorStatusTerminated {
		// 如果自身已经死亡，应该立即通知监视者
		onWatchStopped := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnWatchStopped(ctx.Ref())
		ctx.Reply(nil)
		ctx.tell(ctx.Sender(), onWatchStopped, UserMessage) // 通过用户消息告知已死
		return
	}
	ctx.addWatcher(ctx.Sender())
	ctx.Reply(nil)
}

func (ctx *actorContextRecipient) onWatchStopped(m OnWatchStopped) {
	target := m.GetRef()
	ctx.getTimingWheel().Stop(getActorWatchTimingLoopTaskKey(target)) // 停止监视心跳定时器
	handlers, _ := ctx.getWatcherHandlers(target)

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
	ctx.deleteWatcherHandlers(target)
}

func (ctx *actorContextRecipient) onKilled() {
	// 子 Actor 终止，释放资源
	child := ctx.Sender()
	ctx.unbindChild(child)

	ctx.refreshTerminateStatus()
}

func (ctx *actorContextRecipient) onKillLog() {
	var reason string
	if ctx.Parent() == nil {
		if onKill, ok := ctx.Message().(OnKill); ok {
			reason = onKill.GetReason()
		}
		if reason == "" {
			ctx.System().shutdownLog(log.String("stage", "stopping"), log.String("info", "guard actor stopped"))
		} else {
			ctx.System().shutdownLog(log.String("stage", "stopping"), log.String("info", "guard actor stopped"), log.String("reason", reason))
		}
	} else {
		if onKill, ok := ctx.Message().(OnKill); ok {
			if reason = onKill.GetReason(); reason == "" {
				ctx.Logger().Debug("ActorDeath", log.String("actor", ctx.Ref().String()))
			} else {
				ctx.Logger().Debug("ActorDeath", log.String("actor", ctx.Ref().String()), log.String("reason", reason))
			}
		}
	}
}
