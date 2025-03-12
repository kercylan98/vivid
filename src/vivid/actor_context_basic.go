package vivid

var (
	_ actorContextBasic = (*actorContextBasicImpl)(nil)
)

func newActorContextBasic(ctx ActorContext, system ActorSystem, ref, parentRef ActorRef, provider ActorProvider) *actorContextBasicImpl {
	return &actorContextBasicImpl{
		ctx:      ctx,
		system:   system,
		ref:      ref,
		parent:   parentRef,
		provider: provider,
		actor:    provider.Provide(),
	}
}

type actorContextBasic interface {
	getRef() ActorRef

	getParent() ActorRef

	getSystem() ActorSystem

	getActor() Actor
}

type actorContextBasicImpl struct {
	ctx      ActorContext
	system   ActorSystem
	ref      ActorRef
	parent   ActorRef
	provider ActorProvider
	actor    Actor
}

func (a *actorContextBasicImpl) getActor() Actor {
	return a.actor
}

func (a *actorContextBasicImpl) getParent() ActorRef {
	return a.parent
}

func (a *actorContextBasicImpl) getRef() ActorRef {
	return a.ref
}

func (a *actorContextBasicImpl) getSystem() ActorSystem {
	return a.system
}
