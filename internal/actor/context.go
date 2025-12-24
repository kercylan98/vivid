package actor

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/mailbox"
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
		if err := preLaunchActor.OnPrelaunch(); err != nil {
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
	if !envelop.System() && atomic.LoadInt32(&c.state) != running {
		return // TODO: 应当死信处理
	}

	c.envelop = envelop
	behavior := c.behaviorStack.Peek()

	switch message := c.envelop.Message().(type) {
	case *vivid.OnLaunch:
		behavior(c)
	case *vivid.OnKill:
		c.onKill(message, behavior)
	case *vivid.OnKilled:
		c.onKilled(message, behavior)
	default:
		behavior(c)
	}

}

func (c *Context) onKill(message *vivid.OnKill, behavior vivid.Behavior) {
	if !atomic.CompareAndSwapInt32(&c.state, running, killing) {
		return
	}

	c.Logger().Debug("receive kill", log.String("path", c.ref.path))
	// 等待所有子 Actor 结束
	for _, child := range c.children {
		c.Logger().Debug("notify child kill", log.String("path", child.GetPath()))
		c.Kill(child, message.Poison, message.Reason)
	}

	// 宣告自己进入死亡中
	behavior(c)

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
	behavior(c)

	// 宣告父节点自身死亡
	c.system.removeActorContext(c)
	if c.parent != nil {
		c.tell(true, c.parent, selfKilledMessage)
	}

	c.Logger().Debug("actor killed", log.String("path", c.ref.GetPath()))
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
