package vivid

import (
	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"strings"
	"time"
)

var _ ActorContext = (*actorContext)(nil)

type context interface {
	// Logger 函数将通过配置的日志提供器返回一个日志记录器
	//
	// 这个日志记录器根据 log.Provider 实现的不同，得到的结果是无法确定的，例如上一次获取到的是一个 Debug 级别的日志记录器，而下一刻
	// 获取到的可能是一个 Info 级别的日志记录器
	//
	// 在使用过程中，通常建议一次消息处理至多获取一次日志记录器，这样可以保证同一消息上下文中日志记录器的一致性
	Logger() log.Logger

	// ActorOf 是一个简洁方便的 Actor 生成函数，它可以使用简单的函数式编程风格来快速创建 Actor 实例并返回其 ActorRef
	//
	// 如果具有更复杂的构建流程，可考虑 ActorOfP、 ActorOfC 或 ActorOfPC 函数
	ActorOf(provider ActorProviderFN, configurator ...ActorConfiguratorFN) ActorRef

	// ActorOfP 支持使用 ActorProvider 接口来创建 Actor 实例并提供函数式配置的 Actor 实例生成函数，它将返回生成实例的 ActorRef
	//
	// 该函数与 ActorOf 结果相同，但它支持使用更灵活的方式来创建 Actor 实例
	ActorOfP(provider ActorProvider, configurator ...ActorConfiguratorFN) ActorRef

	// ActorOfC 支持使用 ActorConfigurator 接口来创建 Actor 实例并提供函数式配置的 Actor 实例生成函数，它将返回生成实例的 ActorRef
	//
	// 该函数与 ActorOf 结果相同，但它支持使用更灵活的方式来创建 Actor 实例
	ActorOfC(provider ActorProviderFN, configurator ...ActorConfigurator) ActorRef

	// ActorOfPC 支持使用 ActorProvider 和 ActorConfigurator 接口来创建 Actor 实例并提供函数式配置的 Actor 实例生成函数，它将返回生成实例的 ActorRef
	//
	// 该函数与 ActorOf 结果相同，但它支持使用最灵活的方式来创建 Actor 实例
	ActorOfPC(provider ActorProvider, configurator ...ActorConfigurator) ActorRef

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
	actor.TimingContext

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

func (c *actorContext) After(name string, duration time.Duration, task timing.Task) {
	c.ctx.TimingContext().After(name, duration, task)
}

func (c *actorContext) Loop(name string, duration, interval time.Duration, times int, task timing.Task) {
	c.ctx.TimingContext().Loop(name, duration, interval, times, task)
}

func (c *actorContext) ForeverLoop(name string, duration, interval time.Duration, task timing.Task) {
	c.ctx.TimingContext().ForeverLoop(name, duration, interval, task)
}

func (c *actorContext) Cron(name string, cron string, task timing.Task) error {
	return c.ctx.TimingContext().Cron(name, cron, task)
}

func (c *actorContext) Stop(name string) {
	c.ctx.TimingContext().Stop(name)
}

func (c *actorContext) Clear() {
	c.ctx.TimingContext().Clear()
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

func (c *actorContext) ActorOf(provider ActorProviderFN, configurator ...ActorConfiguratorFN) ActorRef {
	return c.ActorOfP(provider, configurator...)
}

func (c *actorContext) ActorOfP(provider ActorProvider, configurator ...ActorConfiguratorFN) ActorRef {
	var cs = make([]ActorConfigurator, len(configurator))
	for i, cfg := range configurator {
		cs[i] = cfg
	}
	return c.ActorOfPC(provider, cs...)
}

func (c *actorContext) ActorOfC(provider ActorProviderFN, configurator ...ActorConfigurator) ActorRef {
	var cs = make([]ActorConfigurator, len(configurator))
	for i, cfg := range configurator {
		cs[i] = cfg
	}
	return c.ActorOfPC(provider, cs...)
}

func (c *actorContext) ActorOfPC(provider ActorProvider, configurator ...ActorConfigurator) ActorRef {
	system := c.ctx.MetadataContext().System()
	if len(configurator) > 0 {
		var cs = make([]ActorConfigurator, len(configurator))
		for i, c := range configurator {
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
