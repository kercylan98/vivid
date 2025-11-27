package actor

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/transparent"
)

var (
	_ vivid.ActorContext           = &Context{}
	_ transparent.TransportContext = &Context{}
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
		mailbox:       mailbox.NewUnboundedMailbox(),
	}

	for _, option := range options {
		option(ctx.options)
	}

	var name = ctx.options.Name
	if name == "" {
		name = fmt.Sprintf("%d", actorIncrementId.Add(1))
	}

	ctx.ref = NewRef(parent.GetAddress(), parent.GetPath()+"/"+name)
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
	return childCtx.Ref()
}

func (c *Context) Tell(recipient vivid.ActorRef, message vivid.Message) {
	envelop := mailbox.NewEnvelopWithTell(message)
	recipientCtx := c.system.findTransportActorContext(recipient.(*Ref))
	recipientCtx.HandleEnvelop(envelop)
}

func (c *Context) Ask(recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) (vivid.Future[vivid.Message], error) {
	var askTimeout = c.options.DefaultAskTimeout
	if len(timeout) > 0 {
		askTimeout = timeout[0]
	}

	futureIns := future.NewFuture[vivid.Message](askTimeout)
	envelop := mailbox.NewEnvelopWithAsk(futureIns, c.Ref(), message)
	recipientCtx := c.system.findTransportActorContext(recipient.(*Ref))
	recipientCtx.HandleEnvelop(envelop)

	return futureIns, nil
}

func (c *Context) HandleEnvelop(envelop vivid.Envelop) {
	c.mailbox.Enqueue(envelop)
}
