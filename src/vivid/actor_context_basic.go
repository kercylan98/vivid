package vivid

func newActorContextBasic(ref, parentRef ActorRef) actorContextBasic {
	return &actorContextBasicImpl{
		ref:    ref,
		parent: parentRef,
	}
}

type actorContextBasic interface {
	Ref() ActorRef

	Parent() ActorRef
}

type actorContextBasicImpl struct {
	ref    ActorRef
	parent ActorRef
}

func (a *actorContextBasicImpl) Parent() ActorRef {
	return a.parent
}

func (a *actorContextBasicImpl) Ref() ActorRef {
	return a.ref
}
