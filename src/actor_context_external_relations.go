package vivid

var (
	_ actorContextExternalRelationsInternal = (*actorContextExternalRelationsImpl)(nil)
)

func newActorContextExternalRelationsImpl(system ActorSystem, ctx ActorContext, parent ActorRef) *actorContextExternalRelationsImpl {
	return &actorContextExternalRelationsImpl{
		system:       system,
		ActorContext: ctx,
		parent:       parent,
	}
}

type actorContextExternalRelationsImpl struct {
	ActorContext
	system ActorSystem // 所属 Actor 系统
	parent ActorRef    // 父 Actor
}

func (ctx *actorContextExternalRelationsImpl) System() ActorSystem {
	return ctx.system
}

func (ctx *actorContextExternalRelationsImpl) Parent() ActorRef {
	return ctx.parent
}
