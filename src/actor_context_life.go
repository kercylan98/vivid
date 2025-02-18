package vivid

import (
	"context"
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

const (
	timingWheelNameWatchFormater = "[watch]%s"
)

var (
	defaultSlowMessageCancel = context.CancelFunc(func() {})
)

var _ actorContextLifeInternal = (*actorContextLifeImpl)(nil)

func newActorContextLifeImpl(system ActorSystem, ctx ActorContext, config ActorOptionsFetcher, provider ActorProvider, parent ActorRef) *actorContextLifeImpl {
	return &actorContextLifeImpl{
		system:       system,
		ActorContext: ctx,
		config:       config,
		provider:     provider,
		actor:        provider.Provide(),
		parent:       parent,
	}
}

type actorContextLifeImpl struct {
	ActorContext
	system              ActorSystem             // 所属 Actor 系统
	parent              ActorRef                // 父 Actor
	provider            ActorProvider           // Actor 提供者
	config              ActorOptionsFetcher     // Actor 配置
	actor               Actor                   // Actor 实例
	children            map[Path]ActorRef       // 子 Actor
	childGuid           int64                   // 子 Actor 当前自增 GUID
	accidentRecord      AccidentRecord          // 当前自身的事故记录
	unfinishedAccidents map[Path]AccidentRecord // 自身负责的且尚未完结的事故记录
	status              atomic.Uint32           // Actor 状态
	restart             bool                    // 是否需要重启
}

func (ctx *actorContextLifeImpl) System() ActorSystem {
	return ctx.system
}

func (ctx *actorContextLifeImpl) Parent() ActorRef {
	return ctx.parent
}

func (ctx *actorContextLifeImpl) getActor() Actor {
	return ctx.actor
}

func (ctx *actorContextLifeImpl) resetActorState() {
	ctx.actor = ctx.provider.Provide()
}

func (ctx *actorContextLifeImpl) getMessageBuilder() RemoteMessageBuilder {
	return ctx.system.getConfig().FetchRemoteMessageBuilder()
}

func (ctx *actorContextLifeImpl) Ref() ActorRef {
	return ctx.getProcessId()
}

func (ctx *actorContextLifeImpl) getSystemConfig() ActorSystemOptionsFetcher {
	return ctx.System().getConfig()
}

func (ctx *actorContextLifeImpl) getConfig() ActorOptionsFetcher {
	return ctx.config
}

func (ctx *actorContextLifeImpl) getNextChildGuid() int64 {
	ctx.childGuid++
	return ctx.childGuid
}

func (ctx *actorContextLifeImpl) bindChild(ref ActorRef) {
	if ctx.children == nil {
		ctx.children = make(map[Path]ActorRef)
	}
	ctx.children[ref.GetPath()] = ref
}

func (ctx *actorContextLifeImpl) unbindChild(ref ActorRef) {
	delete(ctx.children, ref.GetPath())
	if len(ctx.children) == 0 {
		ctx.children = nil
	}
}

func (ctx *actorContextLifeImpl) getChildren() map[Path]ActorRef {
	return ctx.children
}

func (ctx *actorContextLifeImpl) onAccident(reason Message) {
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

func (ctx *actorContextLifeImpl) onAccidentRecord(record AccidentRecord) {
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

func (ctx *actorContextLifeImpl) clearUnfinishedAccidents() {
	for _, record := range ctx.unfinishedAccidents {
		record.Kill(record.GetVictim(), "be kill, interrupt accident")
	}
}

func (ctx *actorContextLifeImpl) recordUnfinishedAccident(record AccidentRecord) {
	if ctx.unfinishedAccidents == nil {
		ctx.unfinishedAccidents = make(map[Path]AccidentRecord)
	}
	ctx.unfinishedAccidents[record.GetVictim().GetPath()] = record
}

func (ctx *actorContextLifeImpl) onAccidentFinished(record AccidentRecord) {
	delete(ctx.unfinishedAccidents, record.GetVictim().GetPath())
}

func (ctx *actorContextLifeImpl) removeAccidentRecord(removedHandler func(record AccidentRecord)) {
	if ctx.accidentRecord != nil {
		removedHandler(ctx.accidentRecord)
		ctx.accidentRecord = nil
	}
}

func (ctx *actorContextLifeImpl) onKill(event OnKill) {
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
		ctx.getActor().OnReceive(ctx)
	}

	// 停止未完成的事故
	ctx.clearUnfinishedAccidents()

	// 终止子 Actor
	ctx.killAllChildren(event)

	// 刷新终止状态
	ctx.refreshTerminateStatus()
}

func (ctx *actorContextLifeImpl) killAllChildren(event OnKill) {
	var messageType = SystemMessage
	if event.IsPoison() {
		messageType = UserMessage
	}
	for _, ref := range ctx.getChildren() {
		ctx.tell(ref, event, messageType)
	}
}

func (ctx *actorContextLifeImpl) refreshTerminateStatus() {
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
func (ctx *actorContextLifeImpl) onKillLog() {
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

func (ctx *actorContextLifeImpl) onKilled() {
	// 子 Actor 终止，释放资源
	child := ctx.Sender()
	ctx.unbindChild(child)

	ctx.refreshTerminateStatus()
}

func (ctx *actorContextLifeImpl) terminated() bool {
	return ctx.status.Load() == actorStatusTerminated
}

func (ctx *actorContextLifeImpl) onReceive() {
	// 慢消息等待计数器
	var slowMessageThreshold = ctx.System().getConfig().FetchSlowMessageThreshold()
	if actorSlowMessageThreshold := ctx.getConfig().FetchSlowMessageThreshold(); actorSlowMessageThreshold > 0 {
		slowMessageThreshold = actorSlowMessageThreshold
	}
	var slowMessageCancel = defaultSlowMessageCancel
	var start time.Time
	if slowMessageThreshold > 0 {
		var slowMessageContext context.Context
		slowMessageContext, slowMessageCancel = context.WithTimeout(context.Background(), slowMessageThreshold)
		var message = ctx.Message()
		go func() {
			select {
			case <-slowMessageContext.Done():
				cost := time.Since(start)
				if cost > slowMessageThreshold {
					ctx.Logger().Warn("actor", log.String("event", "slow message"), log.String("ref", ctx.Ref().String()), log.Duration("cost", cost), log.Any(fmt.Sprintf("message[%T]", message), message))
				}
			}
		}()
	}

	// 交由用户处理的消息需保证异常捕获
	defer func() {
		slowMessageCancel()
		if reason := recover(); reason != nil {
			switch m := ctx.Message().(type) {
			case OnKill:
				// 如果是 OnKill 中发生了异常，无需继续由监管策略处理，而是继续执行本该执行的停止逻辑
				// 终止可能存在一些释放资源的逻辑，也需要提供消息使得用户能够感知
				ctx.Logger().Error("actor", log.String("event", "kill"), log.String("ref", ctx.Ref().String()), log.String("reason", fmt.Sprint(reason)))

				onKillFailed := ctx.getMessageBuilder().BuildStandardEnvelope(ctx.Ref(), ctx.Ref(), UserMessage,
					ctx.getMessageBuilder().BuildOnKillFailed(debug.Stack(), reason, ctx.Sender(), m),
				)
				ctx.onReceiveEnvelope(onKillFailed)
			case OnKillFailed:
				// 如果是 OnKillFailed 中发生了异常，记录日志
				ctx.Logger().Error("actor", log.String("event", "kill failed"), log.String("ref", ctx.Ref().String()), log.String("reason", fmt.Sprint(reason)))
			default:
				// 其他类型消息交由监管策略执行
				ctx.onAccident(reason)
			}
		}
	}()

	start = time.Now()

	ctx.getActor().OnReceive(ctx)

	switch ctx.Message().(type) {
	case OnLaunch:
		ctx.removeAccidentRecord(func(record AccidentRecord) {
			ctx.Logger().Debug("actor", log.String("event", "restarted"), log.String("ref", ctx.Ref().String()))
		})
	}
}

func (ctx *actorContextLifeImpl) onReceiveEnvelope(envelope Envelope) {
	curr := ctx.getEnvelope()
	defer ctx.setEnvelope(curr)
	ctx.setEnvelope(envelope)
	ctx.onReceive()

}
