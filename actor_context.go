package vivid

import "time"

type ActorContext interface {
	actorCore

	// System 返回当前 ActorContext 所属的 ActorSystem 实例。
	System() ActorSystem

	// Message 返回当前 ActorContext 正在处理的消息。
	Message() Message

	// Sender 返回当前消息的发送者 ActorRef。
	Sender() ActorRef

	// Reply 向消息的发送者回复指定消息。
	Reply(message Message)

	// Become 用新的行为替换当前行为（推入栈顶），进入新的行为状态。
	Become(behavior Behavior)

	// RevertBehavior 将行为恢复到上一个（弹出栈顶），返回是否成功恢复。
	// 如果当前行为栈为空，则返回 false。
	RevertBehavior() bool
}

type actorCore interface {
	// Parent 返回父级 Actor 的 ActorRef。如果该 ActorContext 为根 Actor，则返回 nil。
	Parent() ActorRef

	// Ref 返回当前 Actor 的 ActorRef。
	Ref() ActorRef

	// ActorOf 在当前上下文下创建一个子 Actor，并返回该子 Actor 的引用（ActorRef）。
	//
	// 参数:
	//   - actor: 子 Actor 实例，必须实现 Actor 接口
	//   - options: 可选参数，通过可变参数形式传递（如 Actor 名称、邮箱配置等）
	//
	// 返回值:
	//   - ActorRef: 新创建的子 Actor 的引用
	//
	// 注意：
	//   - 该方法非并发安全，不适用于多协程并发调用。
	//   - 一般情况下，Actor 的创建应仅由其父 ActorContext 进行，天然具备线程安全性。
	ActorOf(actor Actor, options ...ActorOption) ActorRef

	// Tell 向指定的 ActorRef 发送消息。
	//
	// 参数:
	//   - recipient: 目标 Actor 的引用（ActorRef）
	//   - message: 待发送的消息（Message）
	//
	// 说明:
	//   - 该方法为单向消息发送（Tell/SEND），不期望回复。
	//   - 消息会异步投递至目标 Actor 的邮箱。
	Tell(recipient ActorRef, message Message)

	// Ask 向指定的 ActorRef 发送请求消息，并返回用于异步接收回复的 Future。
	//
	// 参数:
	//   - recipient: 目标 Actor 的引用（ActorRef）
	//   - message: 待请求的消息（Message）
	//   - timeout: （可选）超时时间，超过该时间未收到回复则 Future 失败，使用默认超时时间可不传
	//
	// 返回值:
	//   - Future[Message]: 表示异步回复的 Future 实例
	//
	// 说明:
	//   - 该方法为带有回复期望的消息发送（Ask/REQ）。
	//   - 超时时间默认由系统配置，可以通过参数自定义。
	Ask(recipient ActorRef, message Message, timeout ...time.Duration) Future[Message]
}
