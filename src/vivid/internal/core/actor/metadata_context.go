package actor

// MetadataContext 是 Actor 的元数据上下文接口
type MetadataContext interface {
	// Ref 返回 Actor 的 ActorRef
	Ref() Ref

	// Parent 返回 Actor 的父 ActorRef
	Parent() Ref

	// System 返回 Actor 的 ActorSystem
	System() System

	// Config 返回 Actor 的配置
	Config() *Config
}
