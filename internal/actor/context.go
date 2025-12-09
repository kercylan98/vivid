package actor

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/internal/transparent"
)

var (
	_ vivid.ActorContext           = (*Context)(nil)
	_ transparent.TransportContext = (*Context)(nil)
	_ vivid.EnvelopHandler         = (*Context)(nil)
)

var (
	actorIncrementId atomic.Uint64
)

func NewContext(system *System, parent *Ref, actor vivid.Actor, options ...vivid.ActorOption) *Context {
	ctx := &Context{
		options: &vivid.ActorOptions{
			DefaultAskTimeout: system.options.DefaultAskTimeout, // 默认继承系统默认的 Ask 超时时间
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

	var name = ctx.options.Name
	if name == "" {
		name = fmt.Sprintf("%d", actorIncrementId.Add(1))
	}

	var parentAddress net.Addr
	var path = "/"
	if parent != nil {
		parentAddress = parent.GetAddress()
		path = parent.GetPath() + "/" + name
	}

	ctx.ref = NewRef(parentAddress, path)
	ctx.behaviorStack.Push(actor.OnReceive)

	return ctx
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
	message       vivid.Message                      // 当前 ActorContext 的消息
	sender        vivid.ActorRef                     // 当前 ActorContext 的 sender
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

func (c *Context) ActorOf(actor vivid.Actor, options ...vivid.ActorOption) vivid.ActorRef {
	if c.children == nil {
		c.children = make(map[vivid.ActorPath]vivid.ActorRef)
	}

	childCtx := NewContext(c.system, c.ref, actor, options...)
	c.children[childCtx.Ref().GetPath()] = childCtx.Ref()
	c.system.appendActorContext(childCtx)

	c.Tell(childCtx.Ref(), messages.NewInternalMessage(messages.OnLaunchMessageType))
	return childCtx.Ref()
}

func (c *Context) Tell(recipient vivid.ActorRef, message vivid.Message) {
	envelop := mailbox.NewEnvelopWithTell(message)
	recipientCtx := c.system.findTransportActorContext(recipient.(*Ref))
	recipientCtx.DeliverEnvelop(envelop)
}

func (c *Context) Ask(recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) (vivid.Future[vivid.Message], error) {
	var askTimeout = c.options.DefaultAskTimeout
	if len(timeout) > 0 {
		askTimeout = timeout[0]
	}

	futureIns := future.NewFuture[vivid.Message](askTimeout)
	envelop := mailbox.NewEnvelopWithAsk(futureIns, c.Ref(), message)
	recipientCtx := c.system.findTransportActorContext(recipient.(*Ref))
	recipientCtx.DeliverEnvelop(envelop)

	return futureIns, nil
}

func (c *Context) DeliverEnvelop(envelop vivid.Envelop) {
	c.mailbox.Enqueue(envelop)
}

func (c *Context) HandleEnvelop(envelop vivid.Envelop) {
	c.message = envelop.Message()
	c.sender = envelop.Sender()
	c.actor.OnReceive(c)
}

func (c *Context) Message() vivid.Message {
	return c.message
}

func (c *Context) Sender() vivid.ActorRef {
	return c.sender
}
