package vivid

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

var _ ActorContext = (*actorContext)(nil)

type ActorContext interface {
	// Ref 获取自身 Actor 的引用
	Ref() ActorRef

	// Sender 获取当前消息的发送者
	Sender() ActorRef

	// Message 获取当前处理的消息
	Message() Message
}

func newActorContext(ctx actor.Context) ActorContext {
	return &actorContext{
		ctx: ctx,
	}
}

type actorContext struct {
	ctx actor.Context
}

func (c *actorContext) Sender() ActorRef {
	return c.ctx.MessageContext().Sender()
}

func (c *actorContext) Message() Message {
	return c.ctx.MessageContext().Message()
}

func (c *actorContext) Ref() ActorRef {
	return c.ctx.MetadataContext().Ref()
}
