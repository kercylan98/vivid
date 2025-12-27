package actor

import (
	"fmt"
	"runtime/debug"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ vivid.SupervisionContext = (*supervisionContext)(nil)
)

// newSupervisionContext 创建一个新的监督上下文。
//
// ref 为发生故障的 Actor 的引用。
// fault 为发生故障的消息。
//
// 返回:
//   - *supervisionContext: 新的监督上下文。
func newSupervisionContext(ref vivid.ActorRef, fault vivid.Message) *supervisionContext {
	if ref == nil {
		panic("ref is nil")
	}
	if fault == nil {
		panic("fault is nil")
	}

	return &supervisionContext{
		child:      ref.ToActorRefs(),
		fault:      fault,
		faultStack: debug.Stack(),
	}
}

// supervise 确认由 supervisor 监督本次故障的后续处理
func supervise(supervisor *Context, supervisionContext *supervisionContext) {
	supervisionContext.supervisorLogger = supervisor.Logger()
	supervisionContext.supervisorChildren = supervisor.Children()
}

type supervisionContext struct {
	supervisorLogger      log.Logger          // 监督者 Actor 的日志记录器
	supervisorChildren    vivid.ActorRefs     // 监督者 Actor 的子 ActorRefs
	child                 vivid.ActorRefs     // 发生故障的 Actor 的子 ActorRefs
	fault                 vivid.Message       // 发生故障的消息
	faultStack            []byte              // 发生故障的堆栈
	decisionReason        string              // 当前监督者的决策原因
	targets               vivid.ActorRefs     // 当前监督上下文的目标 ActorRefs
	subSupervisionContext *supervisionContext // 下级故障监督上下文
}

// broadcastAllTargets 广播消息到当前监督上下文及所有下级监督上下文的目标 Actor
func (c *supervisionContext) broadcastAllTargets(ctx *Context, message vivid.Message) {
	current := c
	for current != nil {
		for _, target := range current.targets {
			ctx.tell(true, target, message)
		}
		current = current.subSupervisionContext
	}
}

func (c *supervisionContext) applyDecision(ctx *Context, targets vivid.ActorRefs, decision vivid.SupervisionDecision, reason string) {
	c.targets = targets
	c.decisionReason = reason

	switch {
	case decision.IsRestart():

	case decision.IsStop():
		// 结束本身会覆盖所有子级，因此无需再结束子级
		isGraceful := decision.IsGraceful()
		for _, target := range targets {
			ctx.Kill(target, isGraceful, reason)
		}
		// 优雅停止的情况下，由于目标邮箱是挂起的，所以无法被执行，还需要对其邮箱进行恢复处理
		if isGraceful {
			c.broadcastAllTargets(ctx, messages.CommandResumeMailbox.Build())
		}
	case decision.IsResume():
		c.broadcastAllTargets(ctx, messages.CommandResumeMailbox.Build())
	case decision.IsEscalate():
		// 升级后视为自身的故障，但是携带了下级故障信息
		// 挂起当前 Actor 的消息处理并且向父级 Actor 发送监督上下文以触发父级 Actor 的监督策略
		ctx.mailbox.Pause()
		subSupervisionContext := newSupervisionContext(ctx.ref, c.fault)
		subSupervisionContext.subSupervisionContext = c
		ctx.tell(true, ctx.parent, subSupervisionContext)
	}
}

func (c *supervisionContext) Logger() log.Logger {
	return c.supervisorLogger
}

func (c *supervisionContext) Child() vivid.ActorRefs {
	return c.child
}

func (c *supervisionContext) Children() vivid.ActorRefs {
	return c.supervisorChildren
}

func (c *supervisionContext) Message() vivid.Message {
	return c.fault
}

func (c *supervisionContext) Fault() vivid.Message {
	return c.fault
}

func (c *supervisionContext) FaultStack() []byte {
	if c.subSupervisionContext == nil {
		return c.faultStack
	}

	var stack []byte
	current := c
	for current != nil {
		stack = append(stack, []byte(fmt.Sprintf("supervisionContext: %p, child: %s\n", current, current.child.First().GetPath()))...)
		stack = append(stack, current.faultStack...)
		current = current.subSupervisionContext
	}
	return stack
}
