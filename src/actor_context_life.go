package vivid

var _ actorContextLifeInternal = (*actorContextLifeImpl)(nil)

func newActorContextLifeImpl(ctx ActorContext, config ActorOptionsFetcher) *actorContextLifeImpl {
	return &actorContextLifeImpl{
		ActorContext: ctx,
		config:       config,
	}
}

type actorContextLifeImpl struct {
	ActorContext
	config    ActorOptionsFetcher // Actor 配置
	root      bool                // 是否是根 Actor
	childGuid int64               // 子 Actor GUID
	children  map[Path]ActorRef   // 子 Actor
}

func (ctx *actorContextLifeImpl) Ref() ActorRef {
	return ctx.getProcessId()
}

func (ctx *actorContextLifeImpl) getSystemConfig() ActorSystemOptionsFetcher {
	return ctx.System().getConfig()
}

func (ctx *actorContextLifeImpl) getConfig() ActorOptionsFetcher {
	return ctx.config
}

func (ctx *actorContextLifeImpl) getNextChildGuid() int64 {
	ctx.childGuid++
	return ctx.childGuid
}

func (ctx *actorContextLifeImpl) bindChild(ref ActorRef) {
	if ctx.children == nil {
		ctx.children = make(map[Path]ActorRef)
	}
	ctx.children[ref.GetPath()] = ref
}

func (ctx *actorContextLifeImpl) unbindChild(ref ActorRef) {
	delete(ctx.children, ref.GetPath())
	if len(ctx.children) == 0 {
		ctx.children = nil
	}
}

func (ctx *actorContextLifeImpl) getChildren() map[Path]ActorRef {
	return ctx.children
}
