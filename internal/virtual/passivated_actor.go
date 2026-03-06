package virtual

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

const (
	passivatedTTLExpiredReference = "passivatedTTLExpired"
)

type (
	_passivatedTTLExpired struct{} // 钝化时间到期消息
)

var (
	_ vivid.Actor = (*passivatedActor)(nil)

	passivatedTTLExpired = new(_passivatedTTLExpired)
)

func newPassivatedActor(actor vivid.Actor, ttl time.Duration) *passivatedActor {
	return &passivatedActor{
		Actor: actor,
		ttl:   ttl,
	}
}

// passivatedActor 对虚拟演员进行封装，在空闲时间后实现自动钝化。
type passivatedActor struct {
	vivid.Actor
	ttl time.Duration
}

func (p *passivatedActor) OnReceive(ctx vivid.ActorContext) {
	// 虽然可以为 ActorContext 增加函数来判定是否是系统消息，但是考虑到特殊情况，继续使用断言处理。
	// 例如优雅 vivid.OnKill 属于用户消息
	switch ctx.Message().(type) {
	case *vivid.OnKill, *vivid.OnKilled:
		// 特定系统消息不做处理
	case *vivid.OnLaunch:
		p.refreshTTL(ctx)
	default:
		p.refreshTTL(ctx)
	}

	p.Actor.OnReceive(ctx)
}

func (p *passivatedActor) refreshTTL(ctx vivid.ActorContext) {
	scheduler := ctx.Scheduler()
	_ = scheduler.Cancel(passivatedTTLExpiredReference)
	if err := scheduler.Once(ctx.Ref(), p.ttl, passivatedTTLExpired, vivid.WithSchedulerReference(passivatedTTLExpiredReference)); err != nil {
		ctx.Logger().Error("failed to schedule passivated TTL expired", log.Any("error", err))
	}
}
