package actor

// GenerateContext 是 Actor 的生成器上下文接口
type GenerateContext interface {
	// GenerateActorContext 生成 Actor 上下文
	GenerateActorContext(system System, parent Context, provider Provider, ctx Config) Context
}
