package cluster

import "github.com/kercylan98/vivid"

var _ vivid.ActorContext = (*singletonActorContext)(nil)

func newSingletonActorContext(ctx vivid.ActorContext) *singletonActorContext {
	c := &singletonActorContext{
		ActorContext: ctx,
	}

	c.init()

	return c
}

// singletonActorContext 是集群单例 Actor 的上下文，用于处理转发消息和回复消息。用于以特殊的方式处理集群单例 Actor 的上下文。
type singletonActorContext struct {
	vivid.ActorContext
	message vivid.Message
	sender  vivid.ActorRef
}

func (c *singletonActorContext) init() {
	switch msg := c.ActorContext.Message().(type) {
	case *singletonForwardedMessage:
		c.message = msg.message
		c.sender = c.resolveForwardedSender(msg)
	default:
		c.message = msg
		c.sender = nil
	}
}

// resolveForwardedSender 从转发消息解析 sender：本地已有则用，否则用 senderAddr+senderPath 创建 ref。
func (c *singletonActorContext) resolveForwardedSender(fm *singletonForwardedMessage) vivid.ActorRef {
	if fm.sender != nil {
		return fm.sender
	}
	if fm.senderAddr == "" && fm.senderPath == "" {
		return nil
	}
	ref, err := c.ActorContext.System().CreateRef(fm.senderAddr, fm.senderPath)
	if err != nil {
		return nil
	}
	fm.sender = ref
	return ref
}

func (c *singletonActorContext) Message() vivid.Message {
	return c.message
}

func (c *singletonActorContext) Sender() vivid.ActorRef {
	return c.sender
}

func (c *singletonActorContext) Reply(message vivid.Message) {
	c.Tell(c.sender, message)
}
