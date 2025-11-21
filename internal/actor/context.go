package actor

import (
	"sync/atomic"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
)

const (
	actorScheme = "vivid://"
)

var (
	_ vivid.ActorContext = &Context{}
)

var (
	actorIncrementId atomic.Uint64
)

func NewContext(system vivid.ActorSystem, parent vivid.ActorRef, actor vivid.Actor, options ...vivid.ActorOption) *Context {
	ctx := &Context{
		system:        system,
		parent:        parent,
		actor:         actor,
		behaviorStack: NewBehaviorStack(),
		mailbox:       mailbox.NewUnboundedMailbox(),
	}

	opts := &vivid.ActorOptions{}
	for _, option := range options {
		option(opts)
	}

	ctx.ref = NewRefWithParent(parent, opts.Name)
	ctx.behaviorStack.Push(actor.OnReceive)

	return ctx
}

type Context struct {
	system        vivid.ActorSystem                  // 当前 ActorContext 所属的 ActorSystem
	parent        vivid.ActorRef                     // 父 Actor 引用，如果为 nil 则表示根 Actor
	ref           vivid.ActorRef                     // 当前 Actor 引用
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

	childCtx := NewContext(c.System(), c.Ref(), actor, options...)
	c.children[childCtx.Ref().GetPath()] = childCtx.Ref()
	return childCtx.Ref()
}
