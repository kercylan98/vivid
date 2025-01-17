package vivid

import (
	"github.com/kercylan98/go-log/log"
	"strconv"
	"time"
)

var (
	_ ActorContext = (*actorContext)(nil) // 确保 actorContext 实现了 ActorContext 接口
)

// ActorContext 是定义了 Actor 完整的上下文。
type ActorContext interface {
	ActorContextSpawner
	ActorContextLogger
	ActorContextLife
	ActorContextExternalRelations
	ActorContextTransport
	ActorContextActions
}

// ActorContextSpawner 是 ActorContext 的子集，它确保了对子 Actor 的生成
//   - 所有的生成函数均无法保证并发安全
type ActorContextSpawner interface {
	// ActorOf 生成 Actor
	ActorOf(provider ActorProvider, configurator ...ActorConfigurator) ActorRef

	// ActorOfFn 函数式生成 Actor
	ActorOfFn(provider ActorProviderFn, configurator ...ActorConfiguratorFn) ActorRef

	// ActorOfConfig 生成 Actor
	ActorOfConfig(provider ActorProvider, config ActorConfiguration) ActorRef

	// ChainOf 通过责任链的方式生成 Actor
	ChainOf(provider ActorProvider) ActorSpawnChain
}

// ActorContextLogger 是 ActorContext 的子集，它确保了对日志的记录
type ActorContextLogger interface {
	// Logger 获取日志记录器
	Logger() log.Logger

	// GetLoggerProvider 获取日志记录器提供者
	GetLoggerProvider() log.Provider
}

// ActorContextLife 是 ActorContext 的子集，它确保了对 Actor 生命周期的控制
type ActorContextLife interface {
	// Ref 获取当前 Actor 的 ActorRef
	Ref() ActorRef
}

// ActorContextExternalRelations 是 ActorContext 的子集，它确保了 Actor 的外界关系
type ActorContextExternalRelations interface {
	// Parent 获取父 Actor 的 ActorRef
	Parent() ActorRef

	// System 获取当前 Actor 所属的 ActorSystem
	System() ActorSystem
}

// ActorContextActions 是 ActorContext 的子集，它定义了 Actor 所支持的动作
type ActorContextActions interface {
	// Kill 忽略一切尚未处理的消息，立即终止目标 Actor
	Kill(target ActorRef, reason ...string)

	// PoisonKill 等待目标 Actor 处理完当前所有消息后终止目标 Actor
	PoisonKill(target ActorRef, reason ...string)

	// Tell 向指定的 Actor 发送消息
	Tell(target ActorRef, message Message)

	// Ask 向目标 Actor 发送消息，并返回一个 Future 用于获取结果。
	//  - 如果 timeout 参数不存在，那么将会在 DefaultFutureTimeout 时间内等待结果。
	Ask(target ActorRef, message Message, timeout ...time.Duration) Future[Message]

	// Watch 监视目标 Actor 的生命周期，当目标 Actor 终止时，会收到 OnWatchStopped 消息。
	// 该函数会向目标 Actor 发送 Watch 消息，目标 Actor 收到 Watch 消息后会将自己加入到监视列表中。
	//  - 如果传入了 handler 函数，那么当目标 Actor 终止时，会调用 handler 函数，而不再投递 OnWatchStopped 消息。
	//  - handler 的调用是在当前 Actor 中作为消息进行处理的。
	//  - 如果 handler 存在多个，那么会按照顺序调用。
	//  - 重复的调用会追加更多的 handler。
	Watch(target ActorRef, handler ...WatchHandler)

	// Unwatch 取消监视目标 Actor 的生命周期
	Unwatch(target ActorRef)
}

// ActorContextTransport 是 ActorContext 的子集，它确保了对 Actor 之间的通信
type ActorContextTransport interface {
	// Sender 获取当前消息的发送者
	Sender() ActorRef

	// Message 获取当前消息的内容
	Message() Message

	// Reply 向消息的发送者回复消息
	//  - 该函数是 Tell 的快捷方式，用于向消息的发送者回复消息
	Reply(message Message)

	// Ping 尝试对目标 Actor 发送 Ping 消息，并返回 Pong 消息。
	Ping(target ActorRef, timeout ...time.Duration) (Pong, error)
}

type actorContext struct {
	*internalActorContext                    // 内部 Actor 上下文
	provider              ActorProvider      // Actor 提供者
	actor                 Actor              // Actor 实例
	config                ActorConfiguration // Actor 配置
	actorSystem           *actorSystem       // 所属的 ActorSystem
	childGuid             int64              // 子 Actor GUID
	children              map[Path]ActorRef  // 子 Actor
	root                  bool               // 是否是根 Actor
	parent                ActorRef           // 父 Actor
}

func (ctx *actorContext) Ping(target ActorRef, timeout ...time.Duration) (pong Pong, err error) {
	return pong, ctx.ask(target, ctx.systemConfig().FetchRemoteMessageBuilder().BuildOnPing(), SystemMessage, timeout...).Adapter(FutureAdapter[Pong](func(p Pong, err error) error {
		p = pong
		return err
	}))
}

func (ctx *actorContext) systemConfig() ActorSystemOptionsFetcher {
	return ctx.actorSystem.config
}

func (ctx *actorContext) System() ActorSystem {
	return ctx.actorSystem
}

func (ctx *actorContext) Tell(target ActorRef, message Message) {
	ctx.tell(target, message, UserMessage)
}

func (ctx *internalActorContext) Ask(target ActorRef, message Message, timeout ...time.Duration) Future[Message] {
	return ctx.ask(target, message, UserMessage, timeout...)
}

func (ctx *actorContext) Reply(message Message) {
	var target = ctx.envelope.GetAgent()
	if target == nil {
		target = ctx.Sender()
	}
	ctx.tell(target, message, UserMessage)
}

// tell 该函数用于向特定目标发送标准的消息，消息将经过包装并投递到目标 Actor 的邮箱中
//   - 该函数在对自身发送消息时会加速投递，避免通过进程管理器进行查找
func (ctx *actorContext) tell(target ActorRef, message Message, messageType MessageType) {
	envelope := ctx.systemConfig().FetchRemoteMessageBuilder().BuildStandardEnvelope(ctx.Ref(), target, messageType, message)

	if ctx.Ref().Equal(target) {
		// 如果目标是自己，那么通过 Send 函数来对消息进行加速
		// 这个过程可避免通过进程管理器进行查找的过程，而是直接将消息发送到自身进程中
		ctx.Send(envelope)
		return
	}

	ctx.sendToProcess(envelope)
}

func (ctx *actorContext) ask(target ActorRef, message Message, messageType MessageType, timeout ...time.Duration) Future[Message] {
	var t = DefaultFutureTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}

	ctx.childGuid++
	futureRef := ctx.ref.Sub("future-" + string(strconv.AppendInt(nil, ctx.childGuid, 10)))
	future := newFuture[Message](ctx.actorSystem, futureRef, t)
	ctx.sendToProcess(ctx.systemConfig().FetchRemoteMessageBuilder().BuildAgentEnvelope(futureRef, ctx.Ref(), target, messageType, message))
	return future
}

func (ctx *actorContext) Kill(target ActorRef, reason ...string) {
	var r string
	if len(reason) > 0 {
		r = reason[0]
	}
	ctx.tell(target, ctx.systemConfig().FetchRemoteMessageBuilder().BuildOnKill(r, ctx.Ref(), false), SystemMessage)
}

func (ctx *actorContext) PoisonKill(target ActorRef, reason ...string) {
	var r string
	if len(reason) > 0 {
		r = reason[0]
	}
	ctx.tell(target, ctx.systemConfig().FetchRemoteMessageBuilder().BuildOnKill(r, ctx.Ref(), true), UserMessage)
}

func (ctx *actorContext) Sender() ActorRef {
	if ctx.envelope == nil {
		return nil
	}
	return ctx.envelope.GetSender()
}

func (ctx *actorContext) Message() Message {
	if ctx.envelope == nil {
		return nil
	}
	return ctx.envelope.GetMessage()
}

func (ctx *actorContext) Ref() ActorRef {
	return ctx.ref
}

func (ctx *actorContext) Parent() ActorRef {
	return ctx.parent
}

func (ctx *actorContext) GetLoggerProvider() log.Provider {
	return ctx.config.FetchLoggerProvider()
}

func (ctx *actorContext) Logger() log.Logger {
	return ctx.config.FetchLogger()
}

func (ctx *actorContext) ActorOf(provider ActorProvider, configurator ...ActorConfigurator) ActorRef {
	config := NewActorConfig(ctx)
	for _, c := range configurator {
		c.Configure(config)
	}
	return ctx.ActorOfConfig(provider, config)
}

func (ctx *actorContext) ActorOfFn(provider ActorProviderFn, configurator ...ActorConfiguratorFn) ActorRef {
	var c = make([]ActorConfigurator, len(configurator))
	for i, f := range configurator {
		c[i] = f
	}
	return ctx.ActorOf(provider, c...)
}

func (ctx *actorContext) ChainOf(provider ActorProvider) ActorSpawnChain {
	return newActorSpawnChain(ctx, provider)
}

func (ctx *actorContext) ActorOfConfig(provider ActorProvider, config ActorConfiguration) ActorRef {
	return actorOf(ctx.actorSystem, ctx, provider, config).Ref()
}

func (ctx *actorContext) Watch(target ActorRef, handlers ...WatchHandler) {
	onWatch := ctx.systemConfig().FetchRemoteMessageBuilder().BuildOnWatch()
	currHandlers, exist := ctx.watchHandlers[target]
	if !exist {
		ctx.tell(target, onWatch, SystemMessage)
	}

	if ctx.watchHandlers == nil {
		ctx.watchHandlers = make(map[ActorRef][]WatchHandler)
	}

	currHandlers = append(currHandlers, handlers...)
	ctx.watchHandlers[target] = currHandlers

	// TODO: 应该还需要 Ping/Pong 机制来保证监视的有效性，避免监视者已经终止但是监视者未收到通知，从而导致资源泄漏
}

func (ctx *actorContext) Unwatch(target ActorRef) {
	if _, exist := ctx.watchHandlers[target]; !exist {
		return
	}

	onUnwatch := ctx.systemConfig().FetchRemoteMessageBuilder().BuildOnUnwatch()
	ctx.tell(target, onUnwatch, SystemMessage)

	delete(ctx.watchHandlers, target)
	if len(ctx.watchHandlers) == 0 {
		ctx.watchHandlers = nil
	}
}
