package vivid

// Actor 定义了 Actor 模型中的基本行为接口。
//
// Actor 是 vivid 框架中的基本计算单元，通过消息传递进行通信。
// 每个 Actor 都有自己的状态和行为，通过 Receive 方法处理接收到的消息。
type Actor interface {
	// Receive 处理接收到的消息。
	//
	// 该方法是 Actor 的核心逻辑，定义了 Actor 如何响应不同类型的消息。
	// 参数 context 提供了 Actor 的运行上下文，包含消息处理所需的各种功能。
	Receive(context ActorContext)
}

// ActorFN 是 Actor 接口的函数式实现。
//
// 允许使用函数直接实现 Actor 行为，简化了简单 Actor 的创建过程。
type ActorFN func(context ActorContext)

// Receive 实现 Actor 接口的 Receive 方法。
func (fn ActorFN) Receive(context ActorContext) {
	fn(context)
}

// ActorProvider 定义了 Actor 实例的提供者接口。
//
// 用于创建 Actor 实例，支持依赖注入和工厂模式。
type ActorProvider interface {
	// Provide 创建并返回一个新的 Actor 实例。
	//
	// 每次调用都应该返回一个新的 Actor 实例。
	Provide() Actor
}

// ActorProviderFN 是 ActorProvider 接口的函数式实现。
//
// 允许使用函数直接实现 Actor 提供者，简化了 Actor 工厂的创建。
type ActorProviderFN func() Actor

// Provide 实现 ActorProvider 接口的 Provide 方法。
func (fn ActorProviderFN) Provide() Actor {
	return fn()
}
