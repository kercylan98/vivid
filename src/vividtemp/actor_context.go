package vividtemp

var (
	_ ActorContext                      = (*actorContextImpl)(nil)
	_ actorContextChildren              = (*actorContextChildrenImpl)(nil)
	_ actorContextMailboxMessageHandler = (*actorContextMailboxMessageHandlerImpl)(nil)
	_ actorContextProcess               = (*actorContextProcessImpl)(nil)
)

func newActorContext(system ActorSystem, ref, parentRef ActorRef, provider ActorProvider, config *ActorConfiguration) *actorContextImpl {
	ctx := new(actorContextImpl)

	ctx.actorContextConfigurationProvider = newActorContextConfigurationProvider(ctx, config)
	ctx.actorContextBasic = newActorContextBasic(ctx, system, ref, parentRef, provider)
	ctx.actorContextChildren = newActorContextChildren(ctx, ctx.actorContextBasic)
	ctx.actorContextMailboxMessageHandler = newActorContextMailboxMessageHandler(ctx, ctx.actorContextBasic)
	ctx.actorContextProcess = newActorContextProcess(ctx, ctx.actorContextBasic, ctx.actorContextConfigurationProvider)
	ctx.actorContextTransport = newActorContextTransport(ctx, ctx.actorContextProcess)

	return ctx
}

type ActorContext interface {
	Ref() ActorRef

	Parent() ActorRef

	System() ActorSystem

	ActorOf(provider ActorProvider, configuration ...ActorConfiguration) ActorRef
}

type actorContextImpl struct {
	actorContextBasic
	actorContextConfigurationProvider
	actorContextChildren
	actorContextMailboxMessageHandler
	actorContextProcess
	actorContextTransport
}

func (a *actorContextImpl) Ref() ActorRef {
	return a.getRef()
}

func (a *actorContextImpl) Parent() ActorRef {
	return a.getParent()
}

func (a *actorContextImpl) System() ActorSystem {
	return a.getSystem()
}

func (a *actorContextImpl) ActorOf(provider ActorProvider, configuration ...ActorConfiguration) ActorRef {
	return a.actorOf(provider, configuration...)
}
