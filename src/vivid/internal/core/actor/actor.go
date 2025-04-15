package actor

// Actor 是 Vivid 中推崇的最小执行单元，同时也是 Actor 模型的核心概念。
//
// 在 Vivid 中，Actor 是一个轻量级的并发执行单元，它可以接收消息并处理这些消息。
// 它是独立的，拥有自己的状态和行为，并且可以与其他 Actor 进行通信。
//
// 在实际使用过程中，Actor 应一切以消息为中心，杜绝状态共享，以避免竞态条件和死锁等问题。
// Actor 只需要关注如何处理接收到的消息，而不需要关心消息的发送者和接收者。
//
// 实现一个 Actor 只需要实现 OnReceive 方法即可，同时也可以使用 Provider 来提供复杂构建的 Actor，以及使用 FN 实现简单的函数式 Actor。
type Actor interface {
	// OnReceive 是 Actor 接收消息的核心方法，它提供了一个完整的 Context 作为 Actor 的上下文。
	//
	// Actor 在接收到消息时会调用此方法来处理消息。在此方法中，Actor 可以访问上下文信息、发送消息、处理状态等。
	// 该方法是 Actor 的主要入口点，所有的消息处理逻辑都应该在这里实现。
	//
	// 该方法是一个阻塞方法，直到消息处理完成后才会返回。
	// Actor 在处理消息时可以使用上下文提供的方法来发送消息、获取 Actor 的状态等。
	// 该方法的实现应该是非阻塞的，以避免阻塞整个 Actor 系统。
	// 在该方法中，Actor 的消息是串行处理的，即使有多个消息同时到达，Actor 也只会处理一个消息。
	// 其他消息会被放入 Actor 的邮箱中，等待处理。
	//
	// 在使用过程中应严格避免将 Context 向外部暴露，否则可能会导致上下文被篡改或泄露，同时引发竞态条件和死锁等问题。
	//
	// 在该函数中，可通过监听 *OnLaunch 消息来感知到 Actor 的启动并进行相应的初始化行为，同时应避免在构造过程中对 Actor 的初始化。
	// 该函数也可以通过监听 *OnKill 消息来感知到 Actor 的关闭并进行相应的清理行为。
	OnReceive(ctx Context)
}

// FN 是一个函数类型，它实现了 Actor 接口，用于提供简单的函数式 Actor。
type FN func(ctx Context)

func (fn FN) OnReceive(ctx Context) {
	fn(ctx)
}

// Provider 是 Actor 的提供者接口，它用于提供一个 Actor 实例。
//
// Provider 接口的实现可以用于创建复杂的 Actor 实例，例如使用依赖注入、工厂模式等方式来创建 Actor 实例。
//
// 在使用过程中应避免 Provider 中提供一个全局的 Actor 实例，否则可能会导致状态共享和竞态条件等问题。
type Provider interface {
	// Provide 返回一个 Actor 实例。
	//
	// 该实例应该拥有独立的状态，而非全局的状态。
	Provide() Actor
}

// ProviderFN 是一个函数类型，它实现了 Provider 接口，用于提供一个 Actor 实例。
//
// ProviderFN 接口的实现可以用于创建简单的 Actor 实例，例如使用函数式编程的方式来创建 Actor 实例。
type ProviderFN func() Actor

func (fn ProviderFN) Provide() Actor {
	return fn()
}
