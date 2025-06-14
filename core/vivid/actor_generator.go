package vivid

import "strconv"

// ActorGenerator 定义了 Actor 生成器的接口。
//
// ActorGenerator 提供了多种方式来创建和配置 Actor 实例。
// 它支持不同的配置模式，从简单的默认创建到复杂的自定义配置。
//
// 支持的创建方式：
//   - Spawn: 使用默认配置创建
//   - FromConfig: 使用完整配置对象创建
//   - WithConfigurators: 使用配置器模式创建
//   - WithOptions: 使用选项模式创建
type ActorGenerator interface {
	// Spawn 使用默认配置创建并启动一个新的 Actor。
	//
	// 这是最简单的 Actor 创建方式，适用于不需要特殊配置的场景。
	// 返回新创建的 Actor 引用。
	Spawn() ActorRef

	// FromConfig 根据指定的配置创建并启动一个新的 Actor。
	//
	// 参数 config 包含了 Actor 的完整配置信息。
	// 返回新创建的 Actor 引用。
	FromConfig(config *ActorConfiguration) ActorRef

	// WithConfigurators 使用配置器模式创建并启动一个新的 Actor。
	//
	// 配置器模式提供了灵活的配置方式，适合复杂的配置需求。
	// 参数 configurators 是一系列配置器函数。
	// 返回新创建的 Actor 引用。
	WithConfigurators(configurators ...ActorConfigurator) ActorRef

	// WithOptions 使用选项模式创建并启动一个新的 Actor。
	//
	// 选项模式是推荐的配置方式，提供了类型安全的配置选项。
	// 参数 options 是一系列配置选项函数。
	// 返回新创建的 Actor 引用。
	WithOptions(options ...ActorOption) ActorRef
}

func newActorGenerator(context *actorContext, provider ActorProvider) ActorGenerator {
	return &actorGenerator{context, provider}
}

func bindActorContext(system *actorSystem, parent, ctx *actorContext) {
	if err := system.registry.RegisterUnit(ctx.ref, ctx); err != nil {
		panic(err)
	}
	var sender = ctx
	if parent != nil {
		parent.bindChild(ctx.ref)
		sender = parent
	}

	system.hooks.trigger(actorLaunchHookType, ctx)

	sender.systemTell(ctx.ref, onLaunchInstance)
}

type actorGenerator struct {
	context  *actorContext
	provider ActorProvider
}

func (g *actorGenerator) FromConfig(config *ActorConfiguration) ActorRef {
	if config.Name == "" {
		g.context.childGuid++
		config.Name = strconv.FormatInt(g.context.childGuid, 10)
	}

	ctx := newActorContext(g.context.system, g.context.ref.Branch(config.Name), g.context.ref, g.provider, config)
	bindActorContext(g.context.system, g.context, ctx)
	return ctx.ref
}

func (g *actorGenerator) WithConfigurators(configurators ...ActorConfigurator) ActorRef {
	var config = NewActorConfiguration()
	for _, c := range configurators {
		c.Configure(config)
	}

	return g.FromConfig(config)
}

func (g *actorGenerator) WithOptions(options ...ActorOption) ActorRef {
	options = append([]ActorOption{}, options...)
	return g.FromConfig(NewActorConfiguration(options...))
}

func (g *actorGenerator) Spawn() ActorRef {
	return g.WithOptions()
}
