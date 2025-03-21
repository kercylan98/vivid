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

// newActorFacade 创建一个 Actor 门面代理
func newActorFacade(system actor.System, ctx actor.Context, provider ActorProvider, configuration ...ActorConfigurator) ActorRef {
	config := newActorConfig()
	for _, c := range configuration {
		c.Configure(config)
	}
	// 创建 Actor 门面代理的提供器，确保每次生成均能够获得全新的 Actor 实例
	var facadeCtx ActorContext
	facadeProvider := actor.ProviderFN(func() actor.Actor {
		// 创建 Actor 门面代理
		facade := &actorFacade{actor: provider.Provide()}
		// 设置 Actor 门面代理的 Actor 方法
		facade.Actor = actor.FN(func(ctx actor.Context) {
			// 内部消息类型转换
			switch msg := ctx.MessageContext().Message().(type) {
			case *actor.OnKill:
				ctx.MessageContext().OnReceiveImplant(&OnKill{m: msg})
			case *actor.OnDead:
				ctx.MessageContext().OnReceiveImplant(&OnDead{m: msg})
			default:
				facade.actor.OnReceive(facadeCtx)
			}
		})
		return facade
	})
	facadeCtx = newActorContext(ctx.GenerateContext().GenerateActorContext(system, ctx, facadeProvider, *config.config))
	return facadeCtx.Ref()
}

// actorFacade 是 Actor 的门面代理，用于在 Actor 的生命周期中调用 Actor 的方法
type actorFacade struct {
	actor.Actor       // 内部 Actor 实例
	actor       Actor // 对外暴露的 Actor 接口实例
}
