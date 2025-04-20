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

// context 是基础上下文接口，定义了 Actor 与其环境交互的基本方法。
// 它提供了日志记录、Actor 创建和消息传递等功能。
type context interface {
	// Logger 函数将通过配置的日志提供器返回一个日志记录器。
	//
	// 这个日志记录器根据 log.Provider 实现的不同，得到的结果是无法确定的，例如上一次获取到的是一个 Debug 级别的日志记录器，而下一刻，
	// 获取到的可能是一个 Info 级别的日志记录器。
	//
	// 在使用过程中，通常建议一次消息处理至多获取一次日志记录器，这样可以保证同一消息上下文中日志记录器的一致性。
	Logger() log.Logger

	// ActorOf 是一个简洁方便的 Actor 生成函数，它可以使用简单的函数式编程风格来快速创建 Actor 实例并返回其 ActorRef。
	//
	// 如果具有更复杂的构建流程，可考虑 ActorOfP、 ActorOfC 或 ActorOfPC 函数。
	ActorOf(provider ActorProviderFN, configurator ...ActorConfiguratorFN) ActorRef

	// ActorOfP 支持使用 ActorProvider 接口来创建 Actor 实例并提供函数式配置的 Actor 实例生成函数，它将返回生成实例的 ActorRef。
	//
	// 该函数与 ActorOf 结果相同，但它支持使用更灵活的方式来创建 Actor 实例。
	ActorOfP(provider ActorProvider, configurator ...ActorConfiguratorFN) ActorRef

	// ActorOfC 支持使用 ActorConfigurator 接口来创建 Actor 实例并提供函数式配置的 Actor 实例生成函数，它将返回生成实例的 ActorRef。
	//
	// 该函数与 ActorOf 结果相同，但它支持使用更灵活的方式来创建 Actor 实例。
	ActorOfC(provider ActorProviderFN, configurator ...ActorConfigurator) ActorRef

	// ActorOfPC 支持使用 ActorProvider 和 ActorConfigurator 接口来创建 Actor 实例并提供函数式配置的 Actor 实例生成函数，它将返回生成实例的 ActorRef。
	//
	// 该函数与 ActorOf 结果相同，但它支持使用最灵活的方式来创建 Actor 实例。
	ActorOfPC(provider ActorProvider, configurator ...ActorConfigurator) ActorRef

	// Tell 向特定的 Actor 发送不可被回复的消息，
	// 这是一种单向通信方式，发送后不等待响应。
	Tell(target ActorRef, message Message)

	// Probe 向特定的 Actor 发送消息并期待回复，
	// 使用该函数发送的消息，回复是可选的。
	Probe(target ActorRef, message Message)

	// Ask 向特定的 Actor 发送消息并等待回复，
	// 返回一个 Future 对象，可以用来获取响应结果。
	Ask(target ActorRef, message Message, timeout ...time.Duration) Future

	// Ping 向特定的 Actor 发送 ping 消息并等待 pong 响应
	//
	// 该函数将直接返回 Pong 结构体和可能的错误，
	// 如果目标 Actor 不可达或者超时，将返回错误。
	//
	// 这个方法可以用来检测网络可用性并获取详细的响应信息。
	Ping(target ActorRef, timeout ...time.Duration) (*Pong, error)

	// Kill 杀死特定的 Actor。
	//
	// 向目标 Actor 发送一个系统级别的 OnKill 消息，触发其终止流程。
	Kill(ref ActorRef, reason ...string)

	// PoisonKill 毒杀特定的 Actor。
	//
	// 向目标 Actor 发送一个用户级别的 OnKill 消息，这种消息会被放入 Actor 的普通消息队列，
	// 与普通 Kill 不同，PoisonKill 会等待 Actor 处理完当前所有消息后才会被处理。
	PoisonKill(ref ActorRef, reason ...string)
}

// ActorContext 是 Actor 与其环境交互的接口。
//
// 它扩展了基础上下文接口和定时上下文接口，提供了更多与当前 Actor 相关的功能，
// 每个 Actor 在处理消息时都会收到一个 ActorContext 实例，用于访问当前消息、发送者信息以及执行各种操作。
type ActorContext interface {
	context
	actor.TimingContext

	// Ref 返回当前 Actor 的引用，可用于将自身引用传递给其他 Actor
	Ref() ActorRef

	// Sender 返回发送当前正在处理的消息的 Actor 引用。
	//
	// 如果消息没有发送者（例如系统消息），可能返回 nil 或特殊的系统 Actor 引用.
	Sender() ActorRef

	// Message 获取当前 Actor 正在处理的消息对象。
	//
	// 在获取到消息对象后可通过类型断言进行消息处理。
	Message() Message

	// Reply 向当前消息的发送者回复消息
	//
	// 这是一个便捷方法，等同于 Tell(Sender(), message)
	Reply(message Message)

	// Watch 监视特定的 Actor 的生命周期结束信号，当被监视的 Actor 结束生命周期时，会收到一个 *OnDead 消息。
	//
	// 这种机制可用于实现故障检测和资源清理
	Watch(ref ActorRef)

	// Unwatch 取消之前通过 Watch 方法设置的监视
	Unwatch(ref ActorRef)
}

// newActorContext 创建一个新的 ActorContext 实例。
//
// 这是一个内部函数，用于将内部的 actor.Context 包装为公开的 ActorContext 接口
//
// 参数:
//   - ctx: 内部的 actor.Context 实例
//
// 返回:
//   - 创建的 ActorContext 实例
func newActorContext(ctx actor.Context) ActorContext {
	return &actorContext{
		ctx: ctx,
	}
}

type actorContext struct {
	ctx actor.Context // 内部上下文实例
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
	return c.ActorOfPC(provider, configurator...)
}

func (c *actorContext) ActorOfPC(provider ActorProvider, configurator ...ActorConfigurator) ActorRef {
	system := c.ctx.MetadataContext().System()
	if len(configurator) > 0 {
		return newActorFacade(system, c.ctx, provider, configurator...)
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

func (c *actorContext) Ping(target ActorRef, timeout ...time.Duration) (*Pong, error) {
	return c.ctx.TransportContext().Ping(target.(actor.Ref), timeout...)
}
