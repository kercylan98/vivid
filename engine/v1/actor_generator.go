package vivid

import "strconv"

type ActorGenerator interface {
	Spawn() ActorRef
	FromConfig(config *ActorConfiguration) ActorRef
	WithConfigurators(configurators ...ActorConfigurator) ActorRef
	WithOptions(options ...ActorOption) ActorRef
}

func newActorGenerator(context *actorContext, provider ActorProvider) ActorGenerator {
	return &actorGenerator{context, provider}
}

func bindActorContext(system *actorSystem, parent, ctx *actorContext) {
	if err := system.registry.RegisterUnit(ctx.ref, ctx); err != nil {
		panic(err)
	}
	if parent != nil {
		parent.bindChild(ctx.ref)
		parent.systemTell(ctx.ref, onLaunchInstance)
	} else {
		ctx.systemTell(ctx.ref, onLaunchInstance)
	}
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
	var config = NewActorConfiguration(WithActorLogger(g.context.Logger()))
	for _, c := range configurators {
		c.Configure(config)
	}

	return g.FromConfig(config)
}

func (g *actorGenerator) WithOptions(options ...ActorOption) ActorRef {
	options = append([]ActorOption{WithActorLogger(g.context.Logger())}, options...)
	return g.FromConfig(NewActorConfiguration(options...))
}

func (g *actorGenerator) Spawn() ActorRef {
	return g.WithOptions(WithActorLogger(g.context.Logger()))
}
