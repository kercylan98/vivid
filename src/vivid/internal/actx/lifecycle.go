package actx

import (
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
	l.TryRefreshTerminateStatus()
}

func (l *Lifecycle) TryRefreshTerminateStatus() {
	// 如果子 Actor 已经全部终止，那么可以继续流程
	if len(l.ctx.RelationContext().Children()) > 0 {
		return
	}

	// 转变状态到已终止
	if !l.status.CompareAndSwap(lifecycleStatusTerminating, lifecycleStatusTerminated) {
		return
	}

	// 通知父 Actor 自己已经终止
	if parentRef := l.ctx.MetadataContext().Parent(); parentRef != nil {
		l.ctx.TransportContext().Probe(parentRef, SystemMessage, actor.OnKilledMessageInstance)
	}

	l.ctx.MetadataContext().Config().LoggerProvider.Provide().Debug("terminated", l.ctx.MetadataContext().Ref().Path())
}
