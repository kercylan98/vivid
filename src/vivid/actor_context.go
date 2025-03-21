package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"strings"
)

var _ ActorContext = (*actorContext)(nil)

type ActorContext interface {
	// Ref 获取自身 Actor 的引用
	Ref() ActorRef

	// Sender 获取当前消息的发送者
	Sender() ActorRef

	// Message 获取当前处理的消息
	Message() Message

	// ActorOf 创建一个新的 Actor
	ActorOf(provider ActorProvider, configuration ...ActorConfigurator) ActorRef

	// Kill 杀死特定的 Actor
	Kill(ref ActorRef, reason ...string)

	// PoisonKill 毒杀特定的 Actor
	PoisonKill(ref ActorRef, reason ...string)

	// Logger 通过配置的日志提供器获取日志记录器
	//
	// 通常建议一次消息处理至多获取一次日志记录器，这样可以保证同一消息上下文中日志记录器的一致性
	Logger() log.Logger

	// Watch 监视特定的 Actor 的生命周期结束信号，当被监视的 Actor 结束生命周期时，会收到一个 *OnDead 消息
	Watch(ref ActorRef)

	// Unwatch 取消监视特定的 Actor
	Unwatch(ref ActorRef)
}

func newActorContext(ctx actor.Context) ActorContext {
	return &actorContext{
		ctx: ctx,
	}
}

type actorContext struct {
	ctx actor.Context
}

func (c *actorContext) ActorOf(provider ActorProvider, configuration ...ActorConfigurator) ActorRef {
	return newActorFacade(c.ctx.MetadataContext().System(), c.ctx, provider, configuration...)
}

func (c *actorContext) Watch(ref ActorRef) {
	c.ctx.RelationContext().Watch(ref.(actor.Ref))
}

func (c *actorContext) Unwatch(ref ActorRef) {
	c.ctx.RelationContext().Unwatch(ref.(actor.Ref))
}

func (c *actorContext) Logger() log.Logger {
	return c.ctx.MetadataContext().Config().LoggerProvider.Provide()
}

func (c *actorContext) Kill(ref ActorRef, reason ...string) {
	c.ctx.TransportContext().Tell(ref.(actor.Ref), actx.SystemMessage, &actor.OnKill{
		Reason:   strings.Join(reason, ", "),
		Operator: c.ctx.MetadataContext().Ref(),
	})
}

func (c *actorContext) PoisonKill(ref ActorRef, reason ...string) {
	c.ctx.TransportContext().Tell(ref.(actor.Ref), actx.UserMessage, &actor.OnKill{
		Reason:   strings.Join(reason, ", "),
		Operator: c.ctx.MetadataContext().Ref(),
		Poison:   true,
	})
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
