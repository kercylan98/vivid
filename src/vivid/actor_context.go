package vivid

var (
	_ ActorContext                      = (*actorContextImpl)(nil)
	_ actorContextChildren              = (*actorContextChildrenImpl)(nil)
	_ actorContextMailboxMessageHandler = (*actorContextMailboxMessageHandlerImpl)(nil)
	_ actorContextProcess               = (*actorContextProcessImpl)(nil)
)

func newActorContext(system ActorSystem, ref, parentRef ActorRef, provider ActorProvider, config *ActorConfiguration) *actorContextImpl {
	ctx := new(actorContextImpl)

	ctx.config = newActorContextConfigurationProvider(ctx, config)
	ctx.children = newActorContextChildren(ctx)
	ctx.base = newActorContextBasic(ctx, system, ref, parentRef, provider)
	ctx.mailboxMessageHandler = newActorContextMailboxMessageHandler(ctx)
	ctx.process = newActorContextProcess(ctx, ctx.base, ctx.config)
	ctx.transport = newActorContextTransport(ctx)

	return ctx
}

type ActorContext interface {
	Ref() ActorRef

	Parent() ActorRef

	System() ActorSystem
}

type actorContextImpl struct {
	base                  actorContextBasic
	config                actorContextConfigurationProvider
	children              actorContextChildren
	mailboxMessageHandler actorContextMailboxMessageHandler
	process               actorContextProcess
	transport             actorContextTransport
}

func (a *actorContextImpl) Ref() ActorRef {
	return a.base.getRef()
}

func (a *actorContextImpl) Parent() ActorRef {
	return a.base.getParent()
}

func (a *actorContextImpl) System() ActorSystem {
	return a.base.getSystem()
}
