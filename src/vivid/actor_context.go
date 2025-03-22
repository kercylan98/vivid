package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"strings"
	"time"
)

var _ ActorContext = (*actorContext)(nil)

type context interface {
	// Logger 通过配置的日志提供器获取日志记录器
	//
	// 通常建议一次消息处理至多获取一次日志记录器，这样可以保证同一消息上下文中日志记录器的一致性
	Logger() log.Logger

	// ActorOf 创建一个新的 Actor，并返回 ActorRef
	ActorOf(provider ActorProviderFN, configuration ...ActorConfiguratorFN) ActorRef

	// Tell 向特定的 Actor 发送不可被回复的消息
	Tell(target ActorRef, message Message)

	// Probe 向特定的 Actor 发送消息并期待回复
	//  - 使用该函数发送的消息，回复是可选的
	Probe(target ActorRef, message Message)

	// Ask 向特定的 Actor 发送消息并等待回复
	Ask(target ActorRef, message Message, timeout ...time.Duration) Future

	// Kill 杀死特定的 Actor
	Kill(ref ActorRef, reason ...string)

	// PoisonKill 毒杀特定的 Actor
	PoisonKill(ref ActorRef, reason ...string)
}

type ActorContext interface {
	context

	// Ref 获取自身 Actor 的引用
	Ref() ActorRef

	// Sender 获取当前消息的发送者
	Sender() ActorRef

	// Message 获取当前处理的消息
	Message() Message

	// Reply 向当前消息的发送者回复消息
	Reply(message Message)

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

func (c *actorContext) Tell(target ActorRef, message Message) {
	c.ctx.TransportContext().Tell(target.(actor.Ref), actx.UserMessage, message)
}

func (c *actorContext) Probe(target ActorRef, message Message) {
	c.ctx.TransportContext().Probe(target.(actor.Ref), actx.UserMessage, message)
}

func (c *actorContext) Ask(target ActorRef, message Message, timeout ...time.Duration) Future {
	return c.ctx.TransportContext().Ask(target.(actor.Ref), actx.UserMessage, message, timeout...)
}

func (c *actorContext) Reply(message Message) {
	c.ctx.TransportContext().Reply(actx.UserMessage, message)
}

func (c *actorContext) ActorOf(provider ActorProviderFN, configuration ...ActorConfiguratorFN) ActorRef {
	system := c.ctx.MetadataContext().System()
	if len(configuration) > 0 {
		var cs = make([]ActorConfigurator, len(configuration))
		for i, c := range configuration {
			cs[i] = c
		}
		return newActorFacade(system, c.ctx, provider, cs...)
	}
	return newActorFacade(system, c.ctx, provider)
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
