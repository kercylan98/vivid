package vivid

import (
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/system"
	"strings"
)

type ActorSystem interface {
	Start() error

	StartP() ActorSystem

	Stop() error

	StopP()

	ActorOf(provider ActorProvider, configuration ...ActorConfigurator) ActorRef

	Tell(target ActorRef, message Message)

	// Kill 杀死特定的 Actor
	Kill(ref ActorRef, reason ...string)

	// PoisonKill 毒杀特定的 Actor
	PoisonKill(ref ActorRef, reason ...string)
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

func (a *actorSystem) Kill(ref ActorRef, reason ...string) {
	ctx := a.system.Context()
	ctx.TransportContext().Tell(ref.(actor.Ref), actx.SystemMessage, &actor.OnKill{
		Reason:   strings.Join(reason, ", "),
		Operator: ctx.MetadataContext().Ref(),
	})
}

func (a *actorSystem) PoisonKill(ref ActorRef, reason ...string) {
	ctx := a.system.Context()
	ctx.TransportContext().Tell(ref.(actor.Ref), actx.UserMessage, &actor.OnKill{
		Reason:   strings.Join(reason, ", "),
		Operator: ctx.MetadataContext().Ref(),
		Poison:   true,
	})
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

func (a *actorSystem) Stop() error {
	return a.system.Stop()
}

func (a *actorSystem) StopP() {
	if err := a.system.Stop(); err != nil {
		panic(err)
	}
}

func (a *actorSystem) ActorOf(provider ActorProvider, configuration ...ActorConfigurator) ActorRef {
	return newActorFacade(a.system, a.system.Context(), provider, configuration...)
}
