package actor

import (
	"fmt"
	"net/url"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/sugar"
)

var (
	_ vivid.ActorContext   = (*Context)(nil)
	_ vivid.EnvelopHandler = (*Context)(nil)
)

var (
	actorIncrementId atomic.Uint64
)

const (
	running int32 = iota
	killing
	killed
)

func NewContext(system *System, parent *Ref, actor vivid.Actor, options ...vivid.ActorOption) (*Context, error) {
	ctx := &Context{
		options: &vivid.ActorOptions{
			DefaultAskTimeout: system.options.DefaultAskTimeout, // 默认继承系统默认的 Ask 超时时间
			Logger:            system.options.Logger,            // 默认继承系统默认的日志记录器
		},
		system:        system,
		parent:        parent,
		actor:         actor,
		behaviorStack: NewBehaviorStack(),
	}

	for _, option := range options {
		option(ctx.options)
	}

	ctx.mailbox = mailbox.NewUnboundedMailbox(256, ctx)

	var parentAddress = system.options.RemotingAdvertiseAddress
	var path = ctx.options.Name
	if path == "" && parent != nil {
		path = fmt.Sprintf("%d", actorIncrementId.Add(1))
	}
	if parent != nil {
		parentAddress = parent.address
		var err error
		path, err = url.JoinPath(parent.path, path)
		if err != nil {
			return nil, err
		}
	} else {
		path = "/"
	}

	ctx.ref = NewRef(parentAddress, path)
	ctx.behaviorStack.Push(actor.OnReceive)

	return ctx, nil
}

type Context struct {
	options       *vivid.ActorOptions                // 当前 ActorContext 的选项
	system        *System                            // 当前 ActorContext 所属的 ActorSystem
	parent        *Ref                               // 父 Actor 引用，如果为 nil 则表示根 Actor
	ref           *Ref                               // 当前 Actor 引用
	actor         vivid.Actor                        // 当前 Actor
	behaviorStack *BehaviorStack                     // 行为栈
	mailbox       vivid.Mailbox                      // 邮箱
	children      map[vivid.ActorPath]vivid.ActorRef // 懒加载的子 Actor 引用
	envelop       vivid.Envelop                      // 当前 ActorContext 的消息
	state         int32                              // 状态
	restarting    *RestartMessage                    // 正在重启的消息
}

func (c *Context) Logger() log.Logger {
	return c.options.Logger
}

func (c *Context) System() vivid.ActorSystem {
	return c.system
}

func (c *Context) Parent() vivid.ActorRef {
	return c.parent
}

func (c *Context) Ref() vivid.ActorRef {
	return c.ref
}

func (c *Context) Mailbox() vivid.Mailbox {
	return c.mailbox
}

func (c *Context) Name() string {
	return c.options.Name
}

func (c *Context) ActorOf(actor vivid.Actor, options ...vivid.ActorOption) *sugar.Result[vivid.ActorRef] {
	var result sugar.ResultContainer[vivid.ActorRef]
	var status = atomic.LoadInt32(&c.state)
	if status == killed {
		return result.Error(fmt.Errorf("actor killed"))
	}
	if preLaunchActor, ok := actor.(vivid.PrelaunchActor); ok {
		if err := preLaunchActor.OnPrelaunch(c); err != nil {
			return result.Error(err)
		}
	}

	if c.children == nil {
		c.children = make(map[vivid.ActorPath]vivid.ActorRef)
	}

	childCtx, err := NewContext(c.system, c.ref, actor, options...)
	if err != nil {
		return sugar.With[vivid.ActorRef](nil, err)
	}

	if c.system.appendActorContext(childCtx) {
		return sugar.With[vivid.ActorRef](nil, fmt.Errorf("already exists"))
	}

	c.children[childCtx.Ref().GetPath()] = childCtx.Ref()

	c.tell(true, childCtx.Ref(), new(vivid.OnLaunch))
	c.Logger().Debug("actor spawned", log.String("path", childCtx.Ref().GetPath()))

	if status == killing {
		c.Kill(childCtx.ref, false, "parent killed")
	}
	return sugar.With(childCtx.Ref(), nil)
}

func (c *Context) Reply(message vivid.Message) {
	c.Tell(c.envelop.Sender(), message)
}

func (c *Context) TellSelf(message vivid.Message) {
	c.mailbox.Enqueue(mailbox.NewEnvelopWithTell(false, c.ref, c.ref, message))
}

func (c *Context) Tell(recipient vivid.ActorRef, message vivid.Message) {
	c.tell(false, recipient, message)
}

func (c *Context) tell(system bool, recipient vivid.ActorRef, message vivid.Message) {
	envelop := mailbox.NewEnvelopWithTell(system, c.ref, recipient, message)
	receiverMailbox := c.system.findMailbox(recipient.(*Ref))
	receiverMailbox.Enqueue(envelop)
}

func (c *Context) Ask(recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) vivid.Future[vivid.Message] {
	return c.ask(false, recipient, message, timeout...)
}

func (c *Context) ask(system bool, recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) vivid.Future[vivid.Message] {
	var askTimeout = c.options.DefaultAskTimeout
	if len(timeout) > 0 {
		askTimeout = timeout[0]
	}

	agentRef := NewAgentRef(c.ref)
	futureIns := future.NewFuture[vivid.Message](askTimeout, func() {
		c.system.removeFuture(agentRef)
	})
	c.system.appendFuture(agentRef, futureIns)

	envelop := mailbox.NewEnvelopWithAsk(system, agentRef.agent, agentRef.ref, recipient, message)
	receiverMailbox := c.system.findMailbox(recipient.(*Ref))
	receiverMailbox.Enqueue(envelop)

	return futureIns
}

func (c *Context) HandleEnvelop(envelop vivid.Envelop) {
	// 如果当前 Actor 状态不是 running，则不处理消息（非系统消息）
	// 如果当前 Actor 状态是 killing，即便是系统消息，也应当死信处理
	if currentState := atomic.LoadInt32(&c.state); (!envelop.System() && currentState != running) || currentState == killing {
		return // TODO: 应当死信处理
	}

	// 处理消息
	c.envelop = envelop
	behavior := c.behaviorStack.Peek()

	switch message := c.envelop.Message().(type) {
	case *vivid.OnLaunch:
		behavior(c)
	case *vivid.OnKill:
		c.onKill(message, behavior)
	case *vivid.OnKilled:
		c.onKilled(message, behavior)
	case *supervisionContext:
		c.onSupervise(message)
	case *messages.NoneArgsCommandMessage:
		c.onCommand(message)
	case *RestartMessage:
		c.onRestart(message, behavior)
	default:
		defer func() {
			if r := recover(); r != nil {
				c.Logger().Error("unexpected error", log.Any("error", r), log.String("error_type", fmt.Sprintf("%T", r)), log.String("stack", string(debug.Stack())))
				c.Failed(r)
			}
		}()
		behavior(c)
	}

}

func (c *Context) onCommand(message *messages.NoneArgsCommandMessage) {
	switch message.Command {
	case messages.CommandPauseMailbox:
		c.mailbox.Pause()
	case messages.CommandResumeMailbox:
		c.mailbox.Resume()
	}
}

func (c *Context) onRestart(message *RestartMessage, behavior vivid.Behavior) {
	if !atomic.CompareAndSwapInt32(&c.state, running, killing) {
		return
	}

	// 标记正在重启
	c.restarting = message
	c.Logger().Debug("receive restart", log.String("path", c.ref.GetPath()), log.String("reason", message.Reason), log.Any("fault", message.Fault), log.String("stack", string(message.Stack)))

	// 执行重启前的预处理
	// 该阶段主要是用户的自定义清理逻辑，发生异常应当记录错误并继续重启
	if preRestartActor, ok := c.actor.(vivid.PreRestartActor); ok {
		message.recoverExec(c.Logger(), "pre restart", false, func() error {
			return preRestartActor.OnPreRestart(c)
		})
	}

	// 结束自身，由于重启已经进行了优雅处理筛选，应当立即执行
	// 覆盖当前消息，以便在 onKill 中使用
	killMessage := &vivid.OnKill{
		Killer: c.ref,
		Poison: message.Poison,
		Reason: message.Reason,
	}
	c.envelop = mailbox.NewEnvelopWithTell(true, c.Sender(), c.ref, killMessage)
	c.onKill(killMessage, behavior)
}

func (c *Context) onKill(message *vivid.OnKill, behavior vivid.Behavior) {
	if (c.restarting == nil && !atomic.CompareAndSwapInt32(&c.state, running, killing)) || atomic.LoadInt32(&c.state) != running {
		return
	}

	c.Logger().Debug("receive kill", log.String("path", c.ref.path), log.Bool("restarting", c.restarting != nil))
	// 等待所有子 Actor 结束，假设是重启，子 Actor 不应该跟随重启，应该由父节点决定是否重启
	for _, child := range c.children {
		c.Logger().Debug("notify child kill", log.String("path", child.GetPath()))
		c.Kill(child, message.Poison, message.Reason)
	}

	// 宣告自己进入死亡中
	if c.restarting != nil {
		// 失败意味着资源可能无法正确释放，但不应阻止新实例的创建。
		// 可能存在资源泄漏，应当记录警告
		logger := c.Logger()
		if !c.restarting.recoverExec(logger, "on kill", false, func() error {
			behavior(c)
			return nil
		}) {
			logger.Warn("restart kill failed; resources may not have been properly released", log.String("path", c.ref.GetPath()), log.String("reason", c.restarting.Reason), log.Any("fault", c.restarting.Fault), log.String("stack", string(c.restarting.Stack)))
		}
	} else {
		behavior(c)
	}

	// 尝试确认死亡
	c.onKilled(&vivid.OnKilled{Ref: c.ref}, behavior)
}

func (c *Context) onKilled(message *vivid.OnKilled, behavior vivid.Behavior) {

	// 处理子 Actor 死亡
	if !message.Ref.Equals(c.ref) {
		delete(c.children, message.Ref.GetPath())
		behavior(c)
	}

	// 如果还有子 Actor，则不处理自身死亡
	if len(c.children) != 0 || !atomic.CompareAndSwapInt32(&c.state, killing, killed) {
		return
	}

	selfKilledMessage := &vivid.OnKilled{Ref: c.ref}
	c.envelop = mailbox.NewEnvelopWithTell(true, c.Sender(), c.ref, selfKilledMessage)
	if c.restarting != nil {
		// 失败意味着资源可能无法正确释放，但不应阻止新实例的创建。
		// 可能存在资源泄漏，应当记录警告
		logger := c.Logger()
		if !c.restarting.recoverExec(logger, "on killed", false, func() error {
			behavior(c)
			return nil
		}) {
			logger.Warn("restart killed failed; resources may not have been properly released", log.String("path", c.ref.GetPath()), log.String("reason", c.restarting.Reason), log.Any("fault", c.restarting.Fault), log.String("stack", string(c.restarting.Stack)))
		}
	} else {
		behavior(c)
	}

	// 宣告父节点自身死亡，重启状态下并非真正死亡，不做移除和通知
	restarting := c.restarting != nil
	if !restarting {
		c.system.removeActorContext(c)
		if c.parent != nil {
			c.tell(true, c.parent, selfKilledMessage)
		}
	}
	c.Logger().Debug("actor killed", log.String("path", c.ref.GetPath()), log.Bool("restarting", restarting))

	if restarting {
		// 如果提供了提供者，则使用提供者提供新的 Actor 实例
		if c.options.Provider != nil {
			c.actor = c.options.Provider.Provide()
		}
		c.behaviorStack.Clear().Push(c.actor.OnReceive)
		c.Logger().Debug("actor restarted", log.String("path", c.ref.GetPath()))

		// 触发重启后的回调
		var success = true
		if restartedActor, ok := c.actor.(vivid.RestartedActor); ok {
			success = c.restarting.recoverExec(c.Logger(), "on restarted", true, func() error {
				return restartedActor.OnRestarted(c)
			})
		}

		// 触发生命周期
		if preLaunchActor, ok := c.actor.(vivid.PrelaunchActor); ok && success {
			success = c.restarting.recoverExec(c.Logger(), "on pre launch", true, func() error {
				return preLaunchActor.OnPrelaunch(c)
			})
		}

		// 当子 Actor 重启失败时，不再通知父 Actor 其死亡，而是让其进入“僵尸状态”，避免异常状态扩散。
		if !success {
			// 记录错误并释放资源
			c.Logger().Error("restart failed; actor is now in zombie state", log.String("path", c.ref.GetPath()))
			c.system.removeActorContext(c)

			// 现有的 ActorRef 缓存中可能持有该邮箱，应当快速排空且进入死信息，避免内存长时间驻留
			c.mailbox.Resume()
		} else {
			c.restarting = nil
			atomic.CompareAndSwapInt32(&c.state, killing, running)
			c.tell(true, c.parent, new(vivid.OnLaunch))
			c.mailbox.Resume()
		}
	}
}

func (c *Context) Message() vivid.Message {
	return c.envelop.Message()
}

func (c *Context) Sender() vivid.ActorRef {
	if agent := c.envelop.Agent(); agent != nil {
		return agent
	}
	return c.envelop.Sender()
}

func (c *Context) Become(behavior vivid.Behavior) {
	c.behaviorStack.Push(behavior)
}

func (c *Context) RevertBehavior() bool {
	if c.behaviorStack.Len() == 1 {
		return false
	}
	c.behaviorStack.Pop()
	return true
}

func (c *Context) Kill(ref vivid.ActorRef, poison bool, reason ...string) {
	// poison 为 true 时，作为用户消息处理，否则作为系统消息处理
	c.tell(!poison, ref, &vivid.OnKill{
		Killer: c.ref,
		Poison: poison,
		Reason: strings.Join(reason, ", "),
	})
}

func (c *Context) Children() vivid.ActorRefs {
	children := make(vivid.ActorRefs, 0, len(c.children))
	for _, child := range c.children {
		children = append(children, child)
	}
	return children
}

func (c *Context) Failed(fault vivid.Message) {
	// 挂起当前 Actor 的消息处理并且向父级 Actor 发送监督上下文以触发父级 Actor 的监督策略
	c.mailbox.Pause()
	supervisionContext := newSupervisionContext(c.ref, fault)
	c.tell(true, c.parent, supervisionContext)
}

func (c *Context) onSupervise(supervisionContext *supervisionContext) {
	supervise(c, supervisionContext)
	var (
		targets  vivid.ActorRefs
		decision vivid.SupervisionDecision
		reason   string
	)

	// 获取影响的目标和决策
	targets, decision, reason = c.options.SupervisionStrategy.Supervise(supervisionContext)

	// 暂停所有目标的邮箱消息处理
	mailboxPauseMessage := messages.CommandPauseMailbox.Build()
	for _, target := range targets {
		c.tell(true, target, mailboxPauseMessage)
	}
	supervisionContext.applyDecision(c, targets, decision, reason)
}
