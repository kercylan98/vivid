package vivid

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

var _ actor.Actor = (*actorFacade)(nil)

// Actor 是系统中的基本计算单元接口，它封装了状态和行为，并通过消息传递与其他 Actor 通信。
//
// 所有 Actor 必须实现 OnReceive 方法来处理接收到的消息。
//
// 如果所需的 Actor 功能简单，可以使用 ActorFN 函数类型来简化实现。
type Actor interface {
	// OnReceive 当 Actor 接收到消息时被调用。
	//
	// 参数 ctx 提供了访问当前消息上下文的能力，包括获取消息内容、发送者信息以及回复消息等功能。
	OnReceive(ctx ActorContext)
}

// ActorFN 是一个函数类型，实现了 Actor 接口。
//
// 它允许使用函数式编程风格来创建 Actor，简化了 Actor 的定义过程。
type ActorFN func(ctx ActorContext)

// OnReceive 实现 Actor 接口的 OnReceive 方法。
//
// 它直接调用函数本身，将上下文传递给函数处理。
func (fn ActorFN) OnReceive(ctx ActorContext) {
	fn(ctx)
}

// ActorProvider 是 Actor 提供者接口。
//
// 它负责创建和提供 Actor 实例，用于延迟 Actor 的实例化。
//
// 如果 ActorProvider 的实现简单，可以使用 ActorProviderFN 函数类型来简化实现。
type ActorProvider interface {
	// Provide 返回一个新的 Actor 实例
	Provide() Actor
}

// ActorProviderFN 是一个函数类型，实现了 ActorProvider 接口
//
// 它允许使用函数式编程风格来创建 ActorProvider，简化了 ActorProvider 的定义过程
type ActorProviderFN func() Actor

// Provide 实现 ActorProvider 接口的 Provide 方法。
//
// 它直接调用函数本身，返回一个新的 Actor 实例
func (fn ActorProviderFN) Provide() Actor {
	return fn()
}

// newActorFacade 创建一个 Actor 门面代理，
// 该函数接收系统实例、父上下文、Actor提供者和配置参数，返回创建的 Actor 引用，
// 门面代理模式用于在 Actor 的外部和内部之间提供一个统一的接口，处理消息转换和生命周期事件。
func newActorFacade(system actor.System, parent actor.Context, provider ActorProvider, configuration ...ActorConfigurator) ActorRef {
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
			// 内部消息类型转换，处理系统消息和用户消息
			switch msg := ctx.MessageContext().Message().(type) {
			case *actor.OnKill:
				ctx.MessageContext().HandleWith(&OnKill{m: msg})
			case *actor.OnDead:
				ctx.MessageContext().HandleWith(&OnDead{m: msg})
			default:
				facade.actor.OnReceive(facadeCtx)
			}
		})
		return facade
	})
	facadeCtx = newActorContext(parent.GenerateContext().GenerateActorContext(system, parent, facadeProvider, *config.config))
	return facadeCtx.Ref()
}

// actorFacade 是 Actor 的门面代理，用于在 Actor 的生命周期中调用 Actor 的方法，
// 它包含两个 Actor 实例：一个是内部的 Actor 实例，一个是对外暴露的 Actor 接口实例，
// 这种设计模式使得内部实现和外部接口可以分离，提高了系统的灵活性和可维护性。
type actorFacade struct {
	actor.Actor       // 内部 Actor 实例，用于与系统核心交互
	actor       Actor // 对外暴露的 Actor 接口实例，用于处理用户定义的消息逻辑
}
