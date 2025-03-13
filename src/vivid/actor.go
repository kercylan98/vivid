package vivid

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

var _ actor.Actor = (*actorFacade)(nil)

type Actor interface {
	OnReceive(ctx ActorContext)
}

type ActorFN func(ctx ActorContext)

func (fn ActorFN) OnReceive(ctx ActorContext) {
	fn(ctx)
}

type ActorProvider interface {
	Provide() Actor
}

type ActorProviderFN func() Actor

func (fn ActorProviderFN) Provide() Actor {
	return fn()
}

type actorFacade struct {
	actor Actor
	ctx   ActorContext
	actor.Actor
}
