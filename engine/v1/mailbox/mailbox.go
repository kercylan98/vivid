// Package mailbox 提供了 Actor 消息邮箱的抽象接口。
//
// 邮箱是 Actor 模型中的核心组件，负责存储和管理 Actor 接收到的消息。
// 它将消息分为系统消息和用户消息两类，并提供不同的处理优先级。
package mailbox

// Mailbox 定义了 Actor 消息邮箱的接口。
//
// 邮箱负责管理 Actor 的消息队列，支持两种类型的消息：
//   - 系统消息：框架内部使用的控制消息，具有更高的优先级
//   - 用户消息：应用程序发送的业务消息
//
// 邮箱还支持挂起和恢复功能，用于控制消息的处理流程。
type Mailbox interface {
	// PushSystemMessage 向邮箱推送一条系统消息。
	//
	// 系统消息具有更高的处理优先级，用于 Actor 生命周期管理。
	// 参数 message 是要推送的系统消息。
	PushSystemMessage(message any)

	// PushUserMessage 向邮箱推送一条用户消息。
	//
	// 用户消息是应用程序的业务消息，在系统消息之后处理。
	// 参数 message 是要推送的用户消息。
	PushUserMessage(message any)

	// PopSystemMessage 从邮箱弹出一条系统消息。
	//
	// 返回队列中的下一条系统消息，如果没有消息则返回 nil。
	PopSystemMessage() (message any)

	// PopUserMessage 从邮箱弹出一条用户消息。
	//
	// 返回队列中的下一条用户消息，如果没有消息则返回 nil。
	PopUserMessage() (message any)

	// GetSystemMessageNum 获取系统消息队列中的消息数量。
	//
	// 返回当前系统消息队列中待处理的消息数量。
	GetSystemMessageNum() int32

	// GetUserMessageNum 获取用户消息队列中的消息数量。
	//
	// 返回当前用户消息队列中待处理的消息数量。
	GetUserMessageNum() int32

	// Suspend 挂起邮箱。
	//
	// 挂起后，邮箱将停止处理新的用户消息，但系统消息仍会被处理。
	// 这通常用于 Actor 停止或重启过程中。
	Suspend()

	// Resume 恢复邮箱。
	//
	// 恢复后，邮箱将重新开始处理用户消息。
	Resume()

	// Suspended 返回邮箱是否被挂起。
	//
	// 返回值：
	//   - true: 邮箱已被挂起，不处理用户消息
	//   - false: 邮箱正常运行，处理所有消息
	Suspended() bool
}

// Provider 定义了邮箱提供者接口。
//
// 用于创建邮箱实例，支持不同类型的邮箱实现。
type Provider interface {
	// Provide 创建并返回一个新的邮箱实例。
	Provide() Mailbox
}

// ProviderFN 是 Provider 接口的函数式实现。
//
// 允许使用函数直接实现邮箱提供者。
type ProviderFN func() Mailbox

// Provide 实现 Provider 接口的 Provide 方法。
func (p ProviderFN) Provide() Mailbox {
	return p()
}
