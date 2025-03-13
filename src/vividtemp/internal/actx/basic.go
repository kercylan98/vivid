package actx

// Basic 是 ActorContext 实现 Actor 特性的基础接口
type Basic interface {
	GetRef() ActorRef

	GetParent() ActorRef

	GetSystem() ActorSystem

	GetActor() Actor
}
