package actor

import "github.com/kercylan98/go-log/log"

// Context 是 Actor 的上下文接口，它由多个子上下文组成。
//
// 在使用过程中应避免将 Context 向外部暴露，否则可能会导致上下文被篡改或泄露，同时引发竞态条件和死锁等问题。
type Context interface {
	// LoggerProvider 是用于获取 Actor 日志提供器的函数，它将根据 Config 中指定的日志提供器获取 Actor 的日志记录器。
	LoggerProvider() log.Provider

	// MetadataContext 是用于获取 Actor 元数据上下文的函数，其中包含了 Actor 的元数据、父级 Actor 的元数据等信息。
	MetadataContext() MetadataContext

	// RelationContext 是 Actor 的关系上下文，它包含了与外部 Actor 建立关系的行为。
	RelationContext() RelationContext

	// GenerateContext 是 Actor 的生成上下文，它包含了 Actor 的生成行为。
	GenerateContext() GenerateContext

	// ProcessContext 是 Actor 的处理上下文，它为 Actor 实现了一个抽象的进程模型，用于对进程消息进行接收及处理。
	ProcessContext() ProcessContext

	// MessageContext 是 Actor 的消息上下文，它包含了 Actor 的消息处理行为。
	MessageContext() MessageContext

	// TransportContext 是 Actor 的传输上下文，它包含了 Actor 的传输行为。
	TransportContext() TransportContext

	// LifecycleContext 是 Actor 的生命周期上下文，它包含了 Actor 的生命周期行为。
	LifecycleContext() LifecycleContext
}
