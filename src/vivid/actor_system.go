package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/system"
	"strings"
	"time"
)

type ActorSystem interface {
	context
	// Start 启动 ActorSystem
	Start() error

	// StartP 启动 ActorSystem，并在启动失败时引发 panic
	StartP() ActorSystem

	// Stop 停止 ActorSystem
	Stop() error

	// StopP 停止 ActorSystem，并在停止失败时引发 panic
	StopP()
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

func (a *actorSystem) Logger() log.Logger {
	return a.system.Context().MetadataContext().Config().LoggerProvider.Provide()
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

func (a *actorSystem) Probe(target ActorRef, message Message) {
	a.system.Context().TransportContext().Probe(target.(actor.Ref), actx.UserMessage, message)
}

func (a *actorSystem) Ask(target ActorRef, message Message, timeout ...time.Duration) Future {
	return a.system.Context().TransportContext().Ask(target.(actor.Ref), actx.UserMessage, message, timeout...)
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

func (a *actorSystem) ActorOf(provider ActorProviderFN, configuration ...ActorConfiguratorFN) ActorRef {
	if len(configuration) > 0 {
		var cs = make([]ActorConfigurator, len(configuration))
		for i, c := range configuration {
			cs[i] = c
		}
		return newActorFacade(a.system, a.system.Context(), provider, cs...)
	}
	return newActorFacade(a.system, a.system.Context(), provider)
}
