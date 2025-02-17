package vivid

import (
	"fmt"
	"github.com/kercylan98/go-log/log"
	"runtime/debug"
	"sync/atomic"
	"time"
)

const (
	actorStatusAlive       uint32 = iota // Actor 存活状态
	actorStatusTerminating               // Actor 正在终止
	actorStatusTerminated                // Actor 已终止
)

type contextFunc func(ctx ActorContext)

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
	status              atomic.Uint32           // Actor 状态
	restart             bool                    // 是否需要重启
	accidentRecord      AccidentRecord          // 当前自身的事故记录
	unfinishedAccidents map[Path]AccidentRecord // 自身负责的且尚未完结的事故记录
}

func (ctx *actorContextRecipient) OnReceiveEnvelope(envelope Envelope) {
	if status := ctx.status.Load(); status == actorStatusTerminated {
		switch envelope.GetMessage().(type) {
		case OnWatch, OnWatchStopped, contextFunc, *accidentFinished:
			// 此类消息在关闭后依旧可能被发送，需要经过处理以达到状态一致，处理中需要确保考虑到 Actor 不同状态下的处理逻辑
		case OnKill, OnUnwatch:
			// 此类消息在关闭后依旧可能被发送，不处理的效果等同于已经处理
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
		ctx.onProcessUserMessageWithActor()
	case OnKill:
		ctx.onKill(m)
	case OnKilled:
		ctx.onKilled()
	case OnWatch:
		ctx.onWatch()
	case OnUnwatch:
		ctx.deleteWatcher(ctx.Sender())
	case OnWatchStopped:
		ctx.onWatchStopped(m)
	case OnPing:
		ctx.Reply(ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildPong(m))
	case AccidentRecord:
		ctx.onAccidentRecord(m)
	case *accidentFinished:
		ctx.onAccidentFinished(m.AccidentRecord)
	case contextFunc:
		m(ctx)
	case accidentTimingTask:
		m(ctx)
	default:
		panic("unknown system message")
	}
}

func (ctx *actorContextRecipient) onProcessUserMessage(envelope Envelope) {
	switch m := envelope.GetMessage().(type) {
	case OnWatchStopped:
		ctx.onWatchStopped(m)
	case OnKill:
		ctx.onKill(m) // 用户消息已被处理，转为终止 Actor
	case TimingTask:
		m.Execute(ctx)
	case contextFunc:
		m(ctx)
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

	switch ctx.Message().(type) {
	case OnLaunch:
		if ctx.accidentRecord != nil {
			ctx.accidentRecord = nil // 启动成功，清除事故记录
			ctx.Logger().Debug("actor", log.String("event", "restarted"), log.String("ref", ctx.Ref().String()))
		}
	}
}

func (ctx *actorContextRecipient) onAccident(reason Message) {
	// 暂停处理用户消息
	ctx.getMailbox().Suspend()

	switch ctx.Message().(type) {
	case OnLaunch:
		// 重启策略执行失败，记录重启次数
		if ctx.accidentRecord != nil {
			// 此时已经有事故记录了，直接写入新的事故信息覆盖后继续处理即可
			ctx.accidentRecord.recordRestartFailed(ctx.Sender(), ctx.Ref(), ctx.Message(), reason, debug.Stack())

			// 处理事故
			ctx.onAccidentRecord(ctx.accidentRecord)
			return
		}
	}

	// 记录事故
	ctx.accidentRecord = newAccidentRecord(ctx.getMailbox(), ctx.Sender(), ctx.Ref(), ctx.Message(), reason, debug.Stack())

	// 处理事故
	ctx.onAccidentRecord(ctx.accidentRecord)
}

func (ctx *actorContextRecipient) onAccidentRecord(record AccidentRecord) {
	// 设置责任人
	record.setResponsiblePerson(ctx)

	// 使用责任人监管人进行决策
	defer func() {
		if reason := recover(); reason != nil {
			// 如果监管者发生异常，那么将事故升级至上级 Actor 处理
			ctx.Logger().Warn("onAccidentRecord", log.String("info", "supervisor decision failed, escalate to parent actor"), log.Any("record", record))
			record.Escalate()
		}
	}()
	supervisor := ctx.getConfig().FetchSupervisor()
	if supervisor == nil {
		record.Escalate() // 如果没有监管者，那么将事故升级至上级 Actor 处理
	} else {
		supervisor.Decision(record)
		if !record.isFinished() {
			record.Escalate() // 监管者不作为，将事故升级至上级 Actor 处理
		} else {
			// 延迟处理的策略，增加未完结事故记录
			if record.isDelayFinished() {
				ctx.recordUnfinishedAccident(record)
			}
		}
	}
}

func (ctx *actorContextRecipient) onKill(event OnKill) {
	// 当 Actor 处于 actorStatusTerminating 状态时，表明 Actor 已经在终止中
	if !ctx.status.CompareAndSwap(actorStatusAlive, actorStatusTerminating) {
		// 转换状态为终止中，如果失败，表面可能已经终止
		// 重复终止一般是在销毁时再次尝试终止导致，该逻辑可避免非幂等影响
		return
	}

	// 暂停邮箱继续处理用户消息
	ctx.getMailbox().Suspend()

	// 记录重启状态
	ctx.restart = event.Restart()

	// 等待用户处理持久化或清理工作
	if !event.Restart() {
		ctx.onProcessUserMessageWithActor()
	}

	// 停止未完成的事故
	ctx.clearUnfinishedAccidents()

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

	// 如果需要重启，那么不再需要通知监视着及父 Actor，避免误认为 Actor 已经终止
	if ctx.restart {
		// 重置 Actor 状态
		ctx.resetActorState()

		// 恢复 Actor 状态
		ctx.restart = false
		ctx.status.Store(actorStatusAlive)

		// 发送 OnLaunch 消息
		var launchContext map[any]any
		if launchContextProvider := ctx.getConfig().FetchLaunchContextProvider(); launchContextProvider != nil {
			launchContext = launchContextProvider.Provide()
		}
		ctx.tell(ctx.Ref(), ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnLaunch(time.Now(), launchContext, true), SystemMessage)

		// 恢复邮箱
		ctx.getMailbox().Resume()

	} else {
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

}

func (ctx *actorContextRecipient) onWatch() {
	if ctx.status.Load() == actorStatusTerminated {
		// 如果自身已经死亡，应该立即通知监视者
		onWatchStopped := ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnWatchStopped(ctx.Ref())
		ctx.Reply(nil)
		if ctx.Sender().Equal(ctx.Ref()) {
			// 此处转为下一条消息进行处理，避免处理器还未添加就执行了
			ctx.tell(ctx.Ref(), onWatchStopped, SystemMessage)
		} else {
			ctx.tell(ctx.Sender(), onWatchStopped, UserMessage) // 通过用户消息告知已死
		}
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
		if onKill, ok := ctx.Message().(OnKill); ok && !onKill.Restart() {
			if reason = onKill.GetReason(); reason == "" {
				ctx.Logger().Debug("actor", log.String("event", "killed"), log.String("ref", ctx.Ref().String()))
			} else {
				ctx.Logger().Debug("actor", log.String("event", "killed"), log.String("ref", ctx.Ref().String()), log.String("reason", reason))
			}
		} else if onKill.Restart() {
			if reason = onKill.GetReason(); reason == "" {
				ctx.Logger().Debug("actor", log.String("event", "restarting"), log.String("ref", ctx.Ref().String()))
			} else {
				ctx.Logger().Debug("actor", log.String("event", "restarting"), log.String("ref", ctx.Ref().String()), log.String("reason", reason))
			}
		}
	}
}

func (ctx *actorContextRecipient) clearUnfinishedAccidents() {
	for _, record := range ctx.unfinishedAccidents {
		record.Kill(record.GetVictim(), "be kill, interrupt accident")
	}
}

func (ctx *actorContextRecipient) recordUnfinishedAccident(record AccidentRecord) {
	if ctx.unfinishedAccidents == nil {
		ctx.unfinishedAccidents = make(map[Path]AccidentRecord)
	}
	ctx.unfinishedAccidents[record.GetVictim().GetPath()] = record
}

func (ctx *actorContextRecipient) onAccidentFinished(record AccidentRecord) {
	delete(ctx.unfinishedAccidents, record.GetVictim().GetPath())
}
