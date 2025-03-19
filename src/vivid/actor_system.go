package vivid

import (
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/system"
)

type ActorSystem interface {
	Start() error

	StartP() ActorSystem

	Shutdown() error

	ShutdownP()

	ActorOf(provider ActorProvider, configuration ...ActorConfigurator) ActorRef

	Tell(target ActorRef, message Message)
}

func NewActorSystem(configurator ...ActorSystemConfigurator) ActorSystem {
	config := newActorSystemConfig()
	for _, c := range configurator {
		c.Configure(config)
	}
	return &actorSystem{system: system.New(*config.config)}
}

type actorSystem struct {
	system actor.System
}

func (a *actorSystem) Tell(target ActorRef, message Message) {
	a.system.Context().TransportContext().Tell(target.(actor.Ref), actx.UserMessage, message)
}

func (a *actorSystem) Start() error {
	return a.system.Run()
}

func (a *actorSystem) StartP() ActorSystem {
	if err := a.system.Run(); err != nil {
		panic(err)
	}
	return a
}

func (a *actorSystem) Shutdown() error {
	return a.system.Shutdown()
}

func (a *actorSystem) ShutdownP() {
	if err := a.system.Shutdown(); err != nil {
		panic(err)
	}
}

func (a *actorSystem) ActorOf(provider ActorProvider, configuration ...ActorConfigurator) ActorRef {
	config := newActorConfig()
	for _, c := range configuration {
		c.Configure(config)
	}
	facade := &actorFacade{
		actor: provider.Provide(),
	}
	facade.ctx = newActorContext(a.system.Context().GenerateContext().GenerateActorContext(a.system, a.system.Context(), actor.ProviderFN(func() actor.Actor {
		return facade
	}), *config.config))
	facade.Actor = actor.FN(func(ctx actor.Context) {
		facade.actor.OnReceive(facade.ctx)
	})
	return facade.ctx.Ref()
}
