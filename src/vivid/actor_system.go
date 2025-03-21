package vivid

import (
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/system"
	"strings"
)

type ActorSystem interface {
	// Start 启动 ActorSystem
	Start() error

	// StartP 启动 ActorSystem，并在启动失败时引发 panic
	StartP() ActorSystem

	// Stop 停止 ActorSystem
	Stop() error

	// StopP 停止 ActorSystem，并在停止失败时引发 panic
	StopP()

	// ActorOf 创建一个新的 Actor，并返回 ActorRef
	ActorOf(provider ActorProviderFN, configuration ...ActorConfiguratorFn) ActorRef

	// Tell 向特定的 Actor 发送不可被回复的消息
	Tell(target ActorRef, message Message)

	// Probe 向特定的 Actor 发送消息并期待回复
	//  - 使用该函数发送的消息，回复是可选的
	Probe(target ActorRef, message Message)

	// Kill 立即终止特定的 Actor 并丢弃所有未处理的消息
	Kill(ref ActorRef, reason ...string)

	// PoisonKill 在 Actor 处理完毕当前所有剩余消息后终止 Actor
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

func (a *actorSystem) Probe(target ActorRef, message Message) {
	a.system.Context().TransportContext().Probe(target.(actor.Ref), actx.UserMessage, message)
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

func (a *actorSystem) ActorOf(provider ActorProviderFN, configuration ...ActorConfiguratorFn) ActorRef {
	if len(configuration) > 0 {
		var cs = make([]ActorConfigurator, len(configuration))
		for i, c := range configuration {
			cs[i] = c
		}
		return newActorFacade(a.system, a.system.Context(), provider, cs...)
	}
	return newActorFacade(a.system, a.system.Context(), provider)
}
