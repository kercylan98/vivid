package actor

import (
	"fmt"
	"net/url"
	"sync/atomic"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
)

func newContextInitializer(ctx *Context, actor vivid.Actor, options ...vivid.ActorOption) *contextInitializer {
	return &contextInitializer{
		ctx:     ctx,
		options: options,
		actor:   actor,
	}
}

type contextInitializer struct {
	ctx     *Context
	options []vivid.ActorOption
	actor   vivid.Actor
}

func (i *contextInitializer) applyOptions() error {
	for _, option := range i.options {
		option(i.ctx.options)
	}
	return nil
}

func (i *contextInitializer) initActor() error {
	i.ctx.actor = i.actor
	return nil
}

func (i *contextInitializer) initRef() error {
	var parentAddress = i.ctx.system.options.RemotingAdvertiseAddress
	if parentAddress == "" {
		parentAddress = LocalAddress
	}
	var path = i.ctx.options.Name
	if path == "" && i.ctx.parent != nil {
		path = fmt.Sprintf("%d", actorIncrementId.Add(1))
	}
	var joinPathErr error
	if i.ctx.parent != nil {
		parentAddress = i.ctx.parent.GetAddress()
		path, joinPathErr = url.JoinPath(i.ctx.parent.GetPath(), path)
	} else {
		path = "/"
	}

	ref, newRefErr := NewRef(parentAddress, path)
	if joinPathErr != nil || newRefErr != nil {
		return vivid.ErrorRefInvalidPath.With(joinPathErr).With(newRefErr)
	}
	i.ctx.ref = ref
	if i.ctx.parent != nil {
		return i.ctx.system.reserveActorContext(i.ctx)
	}
	return nil
}

// rollbackInitialization 只清理本次创建占有的资源，不触发尚未启动 Actor 的终止行为。
func (c *Context) rollbackInitialization() {
	if c.ref == nil {
		return
	}
	system := c.system
	system.actorInitLock.Lock()
	owned := c.parent == nil || system.initializingActors[c.ref.GetPath()] == c
	system.actorInitLock.Unlock()
	if !owned {
		return
	}
	// 名称保留到清理结束，失败回调持有的上下文也不能再次登记请求。
	atomic.StoreInt32(&c.state, killed)
	system.closeContextRequests(c)
	c.EventStream().UnsubscribeAll(c)
	c.scheduler.Load().close()
	c.phaseKill = nil
	system.actorInitLock.Lock()
	delete(system.initializingActors, c.ref.GetPath())
	system.actorInitLock.Unlock()
}

func (i *contextInitializer) prelaunch() error {
	if preLaunchActor, ok := i.ctx.actor.(vivid.PrelaunchActor); ok {
		if err := preLaunchActor.OnPrelaunch(i.ctx); err != nil {
			return vivid.ErrorActorPrelaunchFailed.With(err)
		}
	}
	return nil
}

func (i *contextInitializer) initMailbox() error {
	i.ctx.mailbox = mailbox.NewUnboundedMailbox(256, i.ctx)
	return nil
}

func (i *contextInitializer) initBehavior() error {
	i.ctx.behaviorStack.Push(i.ctx.actor.OnReceive)
	return nil
}
