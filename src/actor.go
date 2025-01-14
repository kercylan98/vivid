package vivid

// Actor 是由用户定义的 Actor 行为接口，他将对 Actor 所收到的消息进行处理
type Actor interface {
	// OnReceive 当 Actor 接收到消息时，将调用此方法
	OnReceive(ctx ActorContext)
}

// ActorProvider 是 Actor 的提供者接口，他将提供 Actor 实例
type ActorProvider interface {
	// Provide 提供一个 Actor 实例
	Provide() Actor
}

// ActorProviderFn 是 ActorProvider 的函数实现
type ActorProviderFn func() Actor

// Provide 提供一个 Actor 实例
func (f ActorProviderFn) Provide() Actor {
	return f()
}
