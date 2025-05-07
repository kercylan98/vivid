package actx

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/accident"
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"runtime/debug"
	"sync/atomic"
)

const (
	lifecycleStatusAlive       uint32 = iota // Actor 存活状态
	lifecycleStatusTerminating               // Actor 正在终止
	lifecycleStatusTerminated                // Actor 已终止
)

var _ actor.LifecycleContext = (*Lifecycle)(nil)

func NewLifecycle(ctx actor.Context) *Lifecycle {
	return &Lifecycle{
		ctx: ctx,
	}
}

type Lifecycle struct {
	ctx                 actor.Context
	status              atomic.Uint32
	unfinishedAccidents map[core.Path]actor.AccidentSnapshot // 自身负责的且尚未完结的事故记录
	restart             bool                                 // 是否重启中
	accidentSnapshot    actor.AccidentSnapshot               // 事故快照
}

func (l *Lifecycle) Accident(reason core.Message) {
	// 暂停处理用户消息
	l.ctx.MetadataContext().Config().Mailbox.Suspend()

	switch l.ctx.MessageContext().Message().(type) {
	case *actor.OnLaunch:
		// 重启策略执行失败，记录重启次数
		if l.accidentSnapshot != nil {
			// 此时已经有事故记录了，直接写入新的事故信息覆盖后继续处理即可
			l.accidentSnapshot.RecordRestartFailed(l.ctx.MessageContext().Sender(), l.ctx.MetadataContext().Ref(), l.ctx.MessageContext().Message(), reason, debug.Stack())

			// 处理事故
			l.HandleAccident(l.accidentSnapshot)
			return
		}
	}

	// 创建事故快照
	l.accidentSnapshot = accident.NewSnapshot(
		l.ctx.MetadataContext().Config().Mailbox,
		l.ctx.MessageContext().Sender(),
		l.ctx.MetadataContext().Ref(),
		l.ctx.MessageContext().Message(),
		reason,
		debug.Stack(),
	)

	// 处理事故
	l.HandleAccident(l.accidentSnapshot)
}

func (l *Lifecycle) HandleAccident(snapshot actor.AccidentSnapshot) {
	// 设置事故责任人
	snapshot.SetResponsiblePerson(l.ctx)

	// 使用责任人监管人进行决策
	defer func() {
		if reason := recover(); reason != nil {
			// 如果监管者发生异常，那么将事故升级至上级 Actor 处理
			l.ctx.MetadataContext().Config().LoggerProvider.Provide().
				Error("Accident",
					log.String("info", "supervisor decision failed, escalate to parent actor"),
					log.Any("snapshot", snapshot),
				)

			snapshot.Escalate()
		}
	}()
	supervisor := l.ctx.MetadataContext().Config().Supervisor
	if supervisor == nil {
		snapshot.Escalate() // 如果没有监管者，那么将事故升级至上级 Actor 处理
	} else {
		supervisor.Decision(snapshot)
		if !snapshot.IsFinished() {
			snapshot.Escalate() // 监管者不作为，将事故升级至上级 Actor 处理
		} else {
			// 延迟处理的策略，增加未完结事故记录
			if snapshot.IsDelayFinished() {
				if l.unfinishedAccidents == nil {
					l.unfinishedAccidents = make(map[core.Path]actor.AccidentSnapshot)
				}
				l.unfinishedAccidents[snapshot.GetVictim().Path()] = snapshot
			}
		}
	}
}

func (l *Lifecycle) AccidentEnd(snapshot actor.AccidentSnapshot) {
	if snapshot.IsFinished() {
		delete(l.unfinishedAccidents, snapshot.GetVictim().Path())
		if len(l.unfinishedAccidents) == 0 {
			l.unfinishedAccidents = nil
		}
		return
	}
}

func (l *Lifecycle) HandleAccidentSnapshot(snapshot actor.AccidentSnapshot) {
	l.HandleAccident(snapshot)
}

func (l *Lifecycle) Status() uint32 {
	return l.status.Load()
}

func (l *Lifecycle) Kill(info *actor.OnKill) {
	if !l.status.CompareAndSwap(lifecycleStatusAlive, lifecycleStatusTerminating) {
		return
	}

	// 暂停邮箱继续处理用户消息
	l.ctx.MetadataContext().Config().Mailbox.Suspend()

	// 记录重启状态
	l.restart = info.Restart

	// 等待用户消息处理完毕
	l.ctx.GenerateContext().Handle()

	// 停止未完成的事故
	for _, record := range l.unfinishedAccidents {
		record.Kill(record.GetVictim(), "be kill, interrupt accident")
	}

	// 终止所有子 Actor
	for _, ref := range l.ctx.RelationContext().Children() {
		if info.Poison {
			l.ctx.TransportContext().Tell(ref, UserMessage, info)
		} else {
			l.ctx.TransportContext().Tell(ref, SystemMessage, info)
		}
	}

	// 尝试刷新自身终止状态
	l.TerminateTest((*actor.OnKilled)(info))
}

func (l *Lifecycle) TerminateTest(info *actor.OnKilled) {
	// 如果子 Actor 已经全部终止，那么可以继续流程
	if len(l.ctx.RelationContext().Children()) > 0 {
		return
	}

	// 转变状态到已终止
	if !l.status.CompareAndSwap(lifecycleStatusTerminating, lifecycleStatusTerminated) {
		return
	}

	// 清除所有已设置的定时器
	// 定时器可能在生命周期中动态设定，避免出现冲突，在死亡和重启时候均需要清除
	l.ctx.TimingContext().Clear()

	// 记录日志
	var meta = l.ctx.MetadataContext()
	var logger = meta.Config().LoggerProvider.Provide()
	if info.Reason != "" {
		logger.Debug("terminated", log.String("ref", meta.Ref().String()), log.String("reason", info.Reason))
	} else {
		logger.Debug("terminated", log.String("ref", meta.Ref().String()))
	}

	// 如果需要重启，那么不再需要通知监视着及父 Actor，避免误认为 Actor 已经终止
	if l.restart {
		// 重置 Actor 的状态
		l.ctx.GenerateContext().ResetActorState()
		l.restart = false
		l.status.Store(lifecycleStatusAlive)

		// 发送重启的启动消息
		l.ctx.TransportContext().Tell(meta.Ref(), SystemMessage, actor.OnRestartMessageInstance)

		// 恢复邮箱继续处理用户消息
		l.ctx.MetadataContext().Config().Mailbox.Resume()
	} else {
		// 通知监视者自己已经终止
		var watchers = l.ctx.RelationContext().Watchers()
		var onDead *actor.OnDead
		if len(watchers) > 0 {
			onDead = &actor.OnDead{Ref: l.ctx.MetadataContext().Ref()}
		}
		l.ctx.RelationContext().ResetWatchers()
		for _, ref := range watchers {
			// 此刻父 Actor 如果已经进入终止流程，将暂停处理新的用户消息，从而导致 OnDead 消息无法被接收
			l.ctx.TransportContext().Tell(ref, UserMessage, onDead)
			logger.Debug("watcher", log.String("ref", meta.Ref().Path()), log.String("notify", ref.String()))
		}

		// 通知父 Actor 自己已经终止
		parentRef := meta.Parent()
		if parentRef != nil {
			l.ctx.TransportContext().Probe(parentRef, SystemMessage, info)
		}

		// 反注册进程信息
		meta.System().Unregister(info.Operator, meta.Ref())
	}

	// 内部处理 OnKilled（守护）
	l.ctx.MessageContext().HandleWith(info)
}
