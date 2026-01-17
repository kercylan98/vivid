package actor

import (
	"fmt"
	"net/url"

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
	if i.ctx.parent != nil {
		parentAddress = i.ctx.parent.address
		var err error
		path, err = url.JoinPath(i.ctx.parent.path, path)
		if err != nil {
			return err
		}
	} else {
		path = "/"
	}

	ref, err := NewRef(parentAddress, path)
	if err != nil {
		return err
	}
	i.ctx.ref = ref
	return nil
}

func (i *contextInitializer) prelaunch() error {
	if preLaunchActor, ok := i.ctx.actor.(vivid.PrelaunchActor); ok {
		if err := preLaunchActor.OnPrelaunch(i.ctx); err != nil {
			return err
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
