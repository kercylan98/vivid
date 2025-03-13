package vivid

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

var _ ActorContext = (*actorContext)(nil)

type ActorContext interface {
	Ref() ActorRef
}

func newActorContext(ctx actor.Context) ActorContext {
	return &actorContext{
		ctx: ctx,
	}
}

type actorContext struct {
	ctx actor.Context
}

func (c *actorContext) Ref() ActorRef {
	return c.ctx.MetadataContext().Ref()
}
