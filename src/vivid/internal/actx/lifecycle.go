package actx

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
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
	ctx    actor.Context
	status atomic.Uint32
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

	// 等待用户消息处理完毕
	l.ctx.GenerateContext().Actor().OnReceive(l.ctx)

	// 终止所有子 Actor
	for _, ref := range l.ctx.RelationContext().Children() {
		if info.Poison {
			l.ctx.TransportContext().Tell(ref, UserMessage, info)
		} else {
			l.ctx.TransportContext().Tell(ref, SystemMessage, info)
		}
	}

	// 尝试刷新自身终止状态
	l.TryRefreshTerminateStatus((*actor.OnKilled)(info))
}

func (l *Lifecycle) TryRefreshTerminateStatus(info *actor.OnKilled) {
	// 如果子 Actor 已经全部终止，那么可以继续流程
	if len(l.ctx.RelationContext().Children()) > 0 {
		return
	}

	// 转变状态到已终止
	if !l.status.CompareAndSwap(lifecycleStatusTerminating, lifecycleStatusTerminated) {
		return
	}

	// 记录日志
	var meta = l.ctx.MetadataContext()
	var logger = meta.Config().LoggerProvider.Provide()
	if info.Reason != "" {
		logger.Debug("terminated", log.String("ref", meta.Ref().String()), log.String("reason", info.Reason))
	} else {
		logger.Debug("terminated", log.String("ref", meta.Ref().String()))
	}

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

	meta.System().Unregister(info.Operator, meta.Ref())

	// 内部处理 OnKilled（守护）
	l.ctx.MessageContext().OnReceiveImplant(info)
}
