package ves

import (
	"reflect"

	"github.com/kercylan98/vivid"
)

// ActorSpawnedEvent 表示 Actor 被创建的事件。
//
// 该事件在 Actor 通过 ActorContext.ActorOf() 方法成功创建并注册到系统后立即发布。
// 此时 Actor 尚未开始处理消息，仅表示 Actor 实例已被创建并分配了 ActorRef。
//
// 使用场景：
//   - 监控 Actor 系统的创建活动
//   - 统计系统中不同类型 Actor 的数量
//   - 实现 Actor 注册表或服务发现机制
type ActorSpawnedEvent struct {
	// ActorRef 被创建的 Actor 的引用
	ActorRef vivid.ActorRef
	// Type Actor 实例的类型，通过 reflect.TypeOf(actor) 获取
	Type reflect.Type
}

// ActorLaunchedEvent 表示 Actor 启动完成的事件。
//
// 该事件在 Actor 处理完 OnLaunch 消息后发布，表示 Actor 已完成初始化并可以开始处理业务消息。
// 与 ActorSpawnedEvent 的区别在于：Spawned 表示创建完成，Launched 表示启动完成。
//
// 使用场景：
//   - 确认 Actor 已完全就绪并可以接收消息
//   - 实现启动依赖关系管理
//   - 监控 Actor 启动耗时
type ActorLaunchedEvent struct {
	// ActorRef 已启动的 Actor 的引用
	ActorRef vivid.ActorRef
	// Type Actor 实例的类型
	Type reflect.Type
}

// ActorKilledEvent 表示 Actor 被杀死的事件。
//
// 该事件在 Actor 完全终止并释放资源后发布，此时 Actor 已从系统中移除。
// 注意：如果 Actor 正在重启，则不会发布此事件，而是发布 ActorRestartedEvent。
//
// 使用场景：
//   - 监控 Actor 的终止活动
//   - 清理与已终止 Actor 相关的资源
//   - 实现 Actor 生命周期追踪
type ActorKilledEvent struct {
	// ActorRef 被终止的 Actor 的引用
	ActorRef vivid.ActorRef
	// Type Actor 实例的类型
	Type reflect.Type
}

// ActorRestartingEvent 表示 Actor 开始重启的事件。
//
// 该事件在 Actor 因故障触发重启流程时发布，发生在 Actor 进入重启状态但尚未创建新实例之前。
// 通常由监督策略（SupervisionStrategy）在 Actor 失败后决定重启时触发。
//
// 使用场景：
//   - 监控系统的故障恢复活动
//   - 分析故障原因和重启频率
//   - 实现故障告警机制
type ActorRestartingEvent struct {
	// ActorRef 正在重启的 Actor 的引用
	ActorRef vivid.ActorRef
	// Type Actor 实例的类型
	Type reflect.Type
	// Reason 重启原因的描述信息
	Reason string
	// Fault 导致重启的故障信息，通常为 panic 的值或错误对象
	Fault any
}

// ActorRestartedEvent 表示 Actor 重启完成的事件。
//
// 该事件在 Actor 成功重启并恢复运行后发布，表示新的 Actor 实例已创建并完成初始化。
// 只有在重启成功时才会发布此事件；如果重启失败，Actor 将进入僵尸状态，不会发布此事件。
//
// 使用场景：
//   - 确认 Actor 已成功从故障中恢复
//   - 统计重启成功率
//   - 实现故障恢复后的通知机制
type ActorRestartedEvent struct {
	// ActorRef 已重启的 Actor 的引用（引用保持不变，但内部实例已更新）
	ActorRef vivid.ActorRef
	// Type Actor 实例的类型
	Type reflect.Type
}

// ActorFailedEvent 表示 Actor 失败的事件。
//
// 该事件在 Actor 处理消息时发生异常（panic）并调用 ActorContext.Failed() 方法时发布。
// 失败后，Actor 将暂停消息处理并通知父 Actor 触发监督策略。
//
// 使用场景：
//   - 实时监控系统中的异常情况
//   - 收集故障信息用于问题诊断
//   - 实现异常告警和通知机制
type ActorFailedEvent struct {
	// ActorRef 发生失败的 Actor 的引用
	ActorRef vivid.ActorRef
	// Type Actor 实例的类型
	Type reflect.Type
	// Fault 导致失败的故障信息，通常为 panic 的值或错误对象
	Fault any
}

// ActorWatchedEvent 表示 Actor 被监听的事件。
//
// 该事件在另一个 Actor 通过 ActorContext.Watch() 方法开始监听该 Actor 的终止事件时发布。
// 监听者将在被监听的 Actor 终止时收到 OnKilled 消息。
//
// 使用场景：
//   - 追踪 Actor 之间的依赖关系
//   - 监控系统中的监听关系
//   - 实现依赖图构建和分析
type ActorWatchedEvent struct {
	// ActorRef 被监听的 Actor 的引用
	ActorRef vivid.ActorRef
	// Watcher 发起监听的 Actor 的引用
	Watcher vivid.ActorRef
}

// ActorUnwatchedEvent 表示 Actor 取消监听的事件。
//
// 该事件在另一个 Actor 通过 ActorContext.Unwatch() 方法取消对该 Actor 的监听时发布。
//
// 使用场景：
//   - 追踪监听关系的解除
//   - 监控系统中监听关系的变化
//   - 实现依赖关系的动态管理
type ActorUnwatchedEvent struct {
	// ActorRef 被取消监听的 Actor 的引用
	ActorRef vivid.ActorRef
	// Watcher 取消监听的 Actor 的引用
	Watcher vivid.ActorRef
}

// ActorMailboxPausedEvent 表示 Actor 邮箱被暂停的事件。
//
// 该事件在 Actor 的邮箱被暂停消息处理时发布。邮箱暂停通常发生在：
//   - Actor 发生故障，等待监督策略处理
//   - 系统主动暂停 Actor 的消息处理
//
// 暂停期间，Actor 将不再处理新的消息，但消息仍会进入邮箱队列。
//
// 使用场景：
//   - 监控系统中被暂停的 Actor
//   - 诊断消息处理阻塞问题
//   - 实现系统健康检查
type ActorMailboxPausedEvent struct {
	// ActorRef 邮箱被暂停的 Actor 的引用
	ActorRef vivid.ActorRef
	// Type Actor 实例的类型
	Type reflect.Type
}

// ActorMailboxResumedEvent 表示 Actor 邮箱恢复处理的事件。
//
// 该事件在 Actor 的邮箱恢复消息处理时发布。邮箱恢复通常发生在：
//   - Actor 从故障中恢复并重启成功
//   - 监督策略决定恢复 Actor 的消息处理
//
// 恢复后，Actor 将重新开始处理邮箱中的消息。
//
// 使用场景：
//   - 确认 Actor 已恢复正常运行
//   - 监控系统恢复活动
//   - 统计暂停和恢复的频率
type ActorMailboxResumedEvent struct {
	// ActorRef 邮箱已恢复的 Actor 的引用
	ActorRef vivid.ActorRef
	// Type Actor 实例的类型
	Type reflect.Type
}
