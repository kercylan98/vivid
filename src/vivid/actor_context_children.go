package vivid

func newActorContextChildren() actorContextChildren {
	return &actorContextChildrenImpl{}
}

type actorContextChildren interface {
	// nextGuid 获取一个新的 Guid
	nextGuid() int64

	// bindChild 绑定一个子 Actor
	bindChild(child ActorRef)

	// unbindChild 解绑一个子 Actor
	unbindChild(child ActorRef)
}

type actorContextChildrenImpl struct {
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
