package vivid

// EventStream 定义了事件流的核心能力接口，支持 Actor 通过发布/订阅模式进行事件驱动编程。
//
//  - 订阅（Subscribe）指定事件类型后，Actor 将收到所有相应事件的推送。
//  - 事件消息（event）会被投递到 Actor 的邮箱（Mailbox），保证事件和普通消息具备同样的处理模型。
//  - 通常情况下，无需手动调用 Unsubscribe 或 UnsubscribeAll；当 Actor 被销毁时，其全部事件订阅会由 Vivid 自动释放，防止内存泄漏。
//  - 建议所有操作均通过 ActorContext 完成，不要在 Actor 生命周期之外持有 EventStream 或订阅状态引用。
//
// 用法示例:
//     ctx.EventStream().Subscribe(ctx, MyEvent{})
//     ctx.EventStream().Publish(ctx, MyEvent{ ... })
type EventStream interface {
	// Subscribe 订阅指定类型事件。事件到达时将自动推送到 Actor 的邮箱中。
	// ctx: Actor 的上下文，建议为当前 ActorContext。
	// event: 标识事件类型的示例实例，仅作为类型识别，内容可为零值。
	Subscribe(ctx ActorContext, event Message)

	// Unsubscribe 取消当前 Actor 对指定事件类型的订阅。大多数场景下无需主动调用，Actor 销毁时会自动释放全部订阅。
	// 仅用于提前释放部分不再关注的事件类型。
	// ctx: Actor 的上下文。
	// event: 标识事件类型的示例实例，仅作为类型识别，内容可为零值。
	Unsubscribe(ctx ActorContext, event Message)

	// Publish 向系统内所有已订阅该事件类型的 Actor 发布事件。事件会被投递到订阅者的邮箱中，保证 Actor 处理模型一致。
	// ctx: 发布者的 ActorContext。
	// event: 真实事件消息实例。
	Publish(ctx ActorContext, event Message)

	// UnsubscribeAll 取消当前 Actor 对所有事件的订阅。通常场景不必直接调用，系统会在 Actor 销毁时自动执行。
	// ctx: 当前 Actor 的上下文。
	UnsubscribeAll(ctx ActorContext)
}
