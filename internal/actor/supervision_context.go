package actor

import (
	"fmt"
	"runtime/debug"

	"github.com/google/uuid"
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
		id:         uuid.New().String(),
		child:      ref.ToActorRefs(),
		fault:      fault,
		faultStack: debug.Stack(),
	}
}

// RestartMessage 表示重启消息，用于通知 Actor 重启（不可远程传输）。
type RestartMessage struct {
	Fault  vivid.Message // 发生故障的原始故障消息
	Stack  []byte        // 发生故障的原始故障堆栈
	Reason string        // 重启原因
	Poison bool          // 是否优雅重启
}

// recoverExec 捕获 panic 的安全执行函数，返回是否成功和 recover 结果
func (m *RestartMessage) recoverExec(logger log.Logger, name string, alarm bool, handler func() error) (success bool) {
	var result any
	defer func() {
		if r := recover(); r != nil {
			result = r
		} else {
			success = true
		}
		if !success {
			var message = fmt.Sprintf("restart process '%s' failed", name)
			var fields = []any{
				log.String("reason", m.Reason),
				log.String("fault_type", fmt.Sprintf("%T", m.Fault)),
				log.String("fault_value", fmt.Sprintf("%v", m.Fault)),
				log.String("fault_stack", string(m.Stack)),
				log.Any("result", result),
			}
			if alarm {
				logger.Error(message, fields...)
			} else {
				logger.Warn(message, fields...)
			}
		}
	}()
	err := handler()
	success = err == nil
	if !success {
		result = err
	}
	return
}

// supervise 确认由 supervisor 监督本次故障的后续处理
func supervise(supervisor *Context, supervisionContext *supervisionContext) {
	supervisionContext.supervisorLogger = supervisor.Logger()
	supervisionContext.supervisorChildren = supervisor.Children()
}

type supervisionContext struct {
	id                    string              // 当前监督上下文的唯一标识
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
func (c *supervisionContext) broadcastAllTargets(ctx *Context, system bool, message vivid.Message) {
	current := c
	for current != nil {
		for _, target := range current.targets {
			ctx.tell(system, target, message)
		}
		current = current.subSupervisionContext
	}
}

func (c *supervisionContext) applyDecision(ctx *Context, targets vivid.ActorRefs, decision vivid.SupervisionDecision, reason string) {
	c.targets = targets
	c.decisionReason = reason

	ctx.Logger().Debug("supervision: apply decision", log.String("id", c.ID()), log.String("decision", decision.String()), log.String("reason", reason), log.Any("targets", targets))

	switch {
	case decision.IsRestart():
		// 发送重启消息，并携带故障信息和堆栈信息，重启会辐射到所有子级，因此无需再对子级发起重启
		isGraceful := decision.IsGraceful()
		restartMessage := &RestartMessage{
			Reason: reason,
			Fault:  c.fault,
			Stack:  c.FaultStack(),
			Poison: isGraceful,
		}
		for _, target := range targets {
			ctx.tell(!isGraceful, target, restartMessage)
		}

		// 优雅重启的情况下，由于目标邮箱是挂起的，所以无法被执行，还需要对其邮箱进行恢复处理
		if isGraceful {
			c.broadcastAllTargets(ctx, true, messages.CommandResumeMailbox.Build())
		}

	case decision.IsStop():
		// 结束本身会覆盖所有子级，因此无需再结束子级
		isGraceful := decision.IsGraceful()
		for _, target := range targets {
			ctx.Kill(target, isGraceful, reason)
		}
		// 优雅停止的情况下，由于目标邮箱是挂起的，所以无法被执行，还需要对其邮箱进行恢复处理
		if isGraceful {
			c.broadcastAllTargets(ctx, true, messages.CommandResumeMailbox.Build())
		}

	case decision.IsResume():
		c.broadcastAllTargets(ctx, true, messages.CommandResumeMailbox.Build())

	case decision.IsEscalate():
		// 升级后视为自身的故障，但是携带了下级故障信息
		// 挂起当前 Actor 的消息处理并且向父级 Actor 发送监督上下文以触发父级 Actor 的监督策略
		ctx.mailbox.Pause()
		subSupervisionContext := newSupervisionContext(ctx.ref, c.fault)
		subSupervisionContext.subSupervisionContext = c
		ctx.tell(true, ctx.parent, subSupervisionContext)
	}
}

func (c *supervisionContext) ID() string {
	return c.id
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
		childPath := "<nil>"
		if first := current.child.First(); first != nil {
			childPath = first.GetPath()
		}
		stack = append(stack, []byte(fmt.Sprintf("supervisionContext: %p, child: %s\n", current, childPath))...)
		stack = append(stack, current.faultStack...)
		current = current.subSupervisionContext
	}
	return stack
}
