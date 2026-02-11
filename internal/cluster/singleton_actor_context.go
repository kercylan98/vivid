package cluster

import "github.com/kercylan98/vivid"

var _ vivid.ActorContext = (*singletonActorContext)(nil)

func newSingletonActorContext(ctx vivid.ActorContext) *singletonActorContext {
	c := &singletonActorContext{
		ActorContext: ctx,
	}

	switch msg := ctx.Message().(type) {
	case *singletonForwardedMessage:
		c.message = msg.message
		if msg.sender != nil {
			c.sender = msg.sender
		} else if msg.senderAddr != "" || msg.senderPath != "" {
			if ref, err := ctx.System().CreateRef(msg.senderAddr, msg.senderPath); err == nil {
				msg.sender = ref
				c.sender = ref
			}
		}
	default:
		c.message = msg
	}

	return c
}

type singletonActorContext struct {
	vivid.ActorContext
	message vivid.Message
	sender  vivid.ActorRef
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
