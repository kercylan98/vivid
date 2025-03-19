package actor

// GenerateContext 是 Actor 的生成器上下文接口
type GenerateContext interface {
	// GenerateActorContext 生成 Actor 上下文
	GenerateActorContext(system System, parent Context, provider Provider, ctx Config) Context

	// ResetActorState 重置 Actor 状态
	ResetActorState()

	// Actor 获取 Actor 实例
	Actor() Actor
}
