package vivid

import (
	"fmt"
)

var (
	DirectiveRestart    = func() SupervisorDirective { v := directive(0); return &v }() // 重启
	DirectiveKill       = func() SupervisorDirective { v := directive(1); return &v }() // 杀死
	DirectivePoisonKill = func() SupervisorDirective { v := directive(2); return &v }() // 毒杀
	DirectiveResume     = func() SupervisorDirective { v := directive(3); return &v }() // 恢复
	DirectiveEscalate   = func() SupervisorDirective { v := directive(4); return &v }() // 升级
)

type Supervisor interface {
	Strategy(fatal *Fatal) SupervisorDirective
}

type SupervisorFN func(fatal *Fatal) SupervisorDirective

func (fn SupervisorFN) Strategy(fatal *Fatal) SupervisorDirective {
	return fn(fatal)
}

type SupervisorProvider interface {
	Provide() Supervisor
}

type SupervisorProviderFN func() Supervisor

func (fn SupervisorProviderFN) Provide() Supervisor {
	return fn()
}

type SupervisorDirective interface {
	directive() uint8
}

type SupervisorDirectiveProvider interface {
	Provide(fatal *Fatal) SupervisorDirective
}

type SupervisorDirectiveProviderFN func(fatal *Fatal) SupervisorDirective

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

type Fatal struct {
	ctx     *actorContext // 出错的 Actor 上下文
	sender  ActorRef      // 发送者
	message Message       // 发生错误的消息
	reason  Message       // 错误原因
	stack   []byte        // 错误堆栈
}

func newFatal(ctx *actorContext, sender ActorRef, message Message, reason Message, stack []byte) *Fatal {
	return &Fatal{
		ctx:     ctx,
		sender:  sender,
		message: message,
		reason:  reason,
		stack:   stack,
	}
}

func (m *Fatal) Sender() ActorRef {
	return m.sender
}

func (m *Fatal) Ref() ActorRef {
	return m.ctx.ref
}

func (m *Fatal) Message() Message {
	return m.message
}

func (m *Fatal) Reason() Message {
	return m.reason
}

func (m *Fatal) Stack() []byte {
	return m.stack
}

func (m *Fatal) string() string {
	return fmt.Sprintf("received fatal message: %v, reason: %v", m.message, m.reason)
}
