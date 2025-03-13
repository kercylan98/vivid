package vividtemp

func newActorContextChildren(ctx ActorContext, basic actorContextBasic) actorContextChildren {
	return &actorContextChildrenImpl{
		ActorContext: ctx,
		basic:        basic,
	}
}

type actorContextChildren interface {
	ActorContext

	// nextGuid 获取一个新的 Guid
	nextGuid() int64

	// bindChild 绑定一个子 Actor
	bindChild(child ActorRef)

	// unbindChild 解绑一个子 Actor
	unbindChild(child ActorRef)

	actorOf(provider ActorProvider, configuration ...ActorConfiguration) ActorRef
}

type actorContextChildrenImpl struct {
	ActorContext
	basic    actorContextBasic
	guid     int64             // Guid 计数器
	children map[Path]ActorRef // 子 Actor 集合
}

func (a *actorContextChildrenImpl) nextGuid() int64 {
	a.guid++
	return a.guid
}

func (a *actorContextChildrenImpl) bindChild(child ActorRef) {
	if a.children == nil {
		a.children = make(map[Path]ActorRef)
	}
	a.children[child.Path()] = child
}

func (a *actorContextChildrenImpl) unbindChild(child ActorRef) {
	if a.children != nil {
		delete(a.children, child.Path())
		if len(a.children) == 0 {
			a.children = nil
		}
	}
}

func (a *actorContextChildrenImpl) actorOf(provider ActorProvider, configuration ...ActorConfiguration) ActorRef {
	var config ActorConfiguration
	if len(configuration) > 0 {
		config = configuration[0]
	} else {
		config = NewActorConfig()
	}
	return a.basic.getSystem().(actorSystemSpawner).actorOf(a.ActorContext, provider, config).Ref()
}
