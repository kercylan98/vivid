package vivid

import "github.com/kercylan98/wasteland/src/wasteland"

var (
	_ actorSystemConfigProvider = (*actorSystemImpl)(nil)
	_ actorSystemSpawner        = (*actorSystemImpl)(nil)
	_ actorSystemProcess        = (*actorSystemImpl)(nil)
)

// ActorSystem 是完整的 Actor 系统的接口，它包含了对于 Actor Model 的完整实现。
//   - Actor 系统是基于 Actor 模式的并发编程模型，负责管理和调度 Actor 实例。
//   - 它提供了创建、监控、发送消息、以及终止 Actor 的功能。
//   - 在 Actor 系统中，所有的操作都是通过消息传递的方式进行的，
//   - 其中每个 Actor 都是独立的计算单元，通过收发消息与其他 Actor 进行交互。
//
// Actor 系统的设计遵循了高并发和低耦合的原则，能够有效地处理大量并发任务，
// 同时避免传统线程模型中的共享状态问题和锁竞争问题。
// 这使得 Actor 系统在需要高并发、分布式计算和容错的场景中非常适用。
type ActorSystem interface {
	// Start 启动 Actor 系统
	Start() error

	// StartP 启动 Actor 系统，并在发生异常时 panic
	StartP() ActorSystem

	// Shutdown 关闭 Actor 系统
	Shutdown() error

	// ShutdownP 关闭 Actor 系统，并在发生异常时 panic
	ShutdownP() ActorSystem
}

type actorSystemProcess interface {
	ActorSystem

	getProcessRegistry() wasteland.ProcessRegistry
}

// actorSystemConfigProvider 是 Actor 系统配置提供者接口，它提供了 ActorSystem 的配置信息。
type actorSystemConfigProvider interface {
	ActorSystem

	// getConfig 获取 Actor 系统的配置
	getConfig() *ActorSystemConfiguration
}

// actorSystemSpawner 是 ActorSystem 的 Actor 生成器，它用于生成 Actor 实例
type actorSystemSpawner interface {
	ActorSystem

	actorOf(parent ActorContext, provider ActorProvider, config ActorConfiguration) ActorContext
}
