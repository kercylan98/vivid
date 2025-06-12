package vivid

import (
	"fmt"
)

var (
	// DirectiveRestart 重启 Actor，清除状态后重新初始化
	DirectiveRestart = func() SupervisorDirective { v := directive(0); return &v }()
	// DirectiveKill 立即终止 Actor，不进行清理
	DirectiveKill = func() SupervisorDirective { v := directive(1); return &v }()
	// DirectivePoisonKill 优雅终止 Actor，等待当前消息处理完成
	DirectivePoisonKill = func() SupervisorDirective { v := directive(2); return &v }()
	// DirectiveResume 恢复 Actor 运行，忽略错误继续处理消息
	DirectiveResume = func() SupervisorDirective { v := directive(3); return &v }()
	// DirectiveEscalate 将错误上报给父 Actor 处理
	DirectiveEscalate = func() SupervisorDirective { v := directive(4); return &v }()
)

// Supervisor 定义了监管者的策略接口。
//
// 监管者负责处理 Actor 运行时发生的错误，决定如何响应这些错误。
// 这是 Actor 模型中容错机制的核心组件。
type Supervisor interface {
	// Strategy 根据错误信息决定采取的监管策略。
	//
	// 参数 fatal 包含了错误的详细信息，包括出错的 Actor、消息和堆栈信息。
	// 返回一个 SupervisorDirective，指示如何处理这个错误。
	Strategy(fatal *Fatal) SupervisorDirective
}

// SupervisorFN 是 Supervisor 接口的函数式实现。
//
// 允许使用函数直接实现监管策略，简化了简单监管逻辑的创建。
type SupervisorFN func(fatal *Fatal) SupervisorDirective

// Strategy 实现 Supervisor 接口的 Strategy 方法。
func (fn SupervisorFN) Strategy(fatal *Fatal) SupervisorDirective {
	return fn(fatal)
}

// SupervisorProvider 定义了监管者提供者接口。
//
// 用于创建监管者实例，支持依赖注入和工厂模式。
type SupervisorProvider interface {
	// Provide 创建并返回一个新的监管者实例。
	Provide() Supervisor
}

// SupervisorProviderFN 是 SupervisorProvider 接口的函数式实现。
//
// 允许使用函数直接实现监管者提供者。
type SupervisorProviderFN func() Supervisor

// Provide 实现 SupervisorProvider 接口的 Provide 方法。
func (fn SupervisorProviderFN) Provide() Supervisor {
	return fn()
}

// SupervisorDirective 定义了监管指令的接口。
//
// 监管指令表示监管者对错误的处理决策。
type SupervisorDirective interface {
	directive() uint8
}

// SupervisorDirectiveProvider 定义了监管指令提供者接口。
//
// 用于根据错误信息动态生成监管指令。
type SupervisorDirectiveProvider interface {
	// Provide 根据错误信息提供相应的监管指令。
	//
	// 参数 fatal 包含了错误的详细信息。
	// 返回适合该错误的监管指令。
	Provide(fatal *Fatal) SupervisorDirective
}

// SupervisorDirectiveProviderFN 是 SupervisorDirectiveProvider 接口的函数式实现。
type SupervisorDirectiveProviderFN func(fatal *Fatal) SupervisorDirective

// Provide 实现 SupervisorDirectiveProvider 接口的 Provide 方法。
func (fn SupervisorDirectiveProviderFN) Provide(fatal *Fatal) SupervisorDirective {
	return fn(fatal)
}

type directive uint8

func (d *directive) directive() uint8 {
	return uint8(*d)
}

func (d *directive) String() string {
	switch d {
	case DirectiveRestart:
		return "restart"
	case DirectiveKill:
		return "kill"
	case DirectivePoisonKill:
		return "poison_kill"
	case DirectiveResume:
		return "resume"
	case DirectiveEscalate:
		return "escalate"
	default:
		return "unknown"
	}
}

// Fatal 表示 Actor 运行时发生的致命错误信息。
//
// Fatal 包含了错误发生时的完整上下文信息，用于监管者做出正确的处理决策。
//
// 包含信息：
//   - 出错的 Actor 上下文
//   - 发送消息的 Actor
//   - 导致错误的消息
//   - 错误原因和堆栈信息
//   - 重启次数统计
type Fatal struct {
	ctx          *actorContext // 出错的 Actor 上下文
	sender       ActorRef      // 发送者
	message      Message       // 发生错误的消息
	reason       Message       // 错误原因
	stack        []byte        // 错误堆栈
	restartCount int           // 当前重启次数
}

func newFatal(ctx *actorContext, sender ActorRef, message Message, reason Message, stack []byte) *Fatal {
	if ctx.fatal != nil {
		ctx.fatal.sender, ctx.fatal.message, ctx.fatal.reason, ctx.fatal.stack = sender, message, reason, stack
		return ctx.fatal
	}
	return &Fatal{
		ctx:     ctx,
		sender:  sender,
		message: message,
		reason:  reason,
		stack:   stack,
	}
}

// Sender 返回发送导致错误的消息的 Actor 引用。
func (m *Fatal) Sender() ActorRef {
	return m.sender
}

// Ref 返回发生错误的 Actor 引用。
func (m *Fatal) Ref() ActorRef {
	return m.ctx.ref
}

// Message 返回导致错误的消息。
func (m *Fatal) Message() Message {
	return m.message
}

// Reason 返回错误的原因。
func (m *Fatal) Reason() Message {
	return m.reason
}

// Stack 返回错误发生时的堆栈信息。
func (m *Fatal) Stack() []byte {
	return m.stack
}

func (m *Fatal) String() string {
	if m == nil {
		return ""
	}
	return fmt.Sprintf("received fatal message: %v, reason: %v", m.message, m.reason)
}

// RestartCount 返回当前 Actor 的重启次数。
//
// 这个信息可以用于实现退避策略或限制重启次数。
func (m *Fatal) RestartCount() int {
	return m.restartCount
}
