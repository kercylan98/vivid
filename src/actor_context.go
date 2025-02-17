package vivid

import (
	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/go-log/log"
	"time"
)

var (
	_ ActorContext = (*actorContext)(nil) // 确保 actorContext 实现了 ActorContext 接口
)

// ActorContext 是定义了 Actor 完整的上下文。
type ActorContext interface {
	actorContextProcessInternal
	actorContextSpawnerInternal
	actorContextLoggerInternal
	actorContextLifeInternal
	actorContextExternalRelationsInternal
	actorContextTransportInternal
	actorContextActionsInternal
	actorContextTimingInternal
}

// ActorContextProcess 是 ActorContext 的子集，它确保了 Actor 与 Process 相关的内容
type (
	ActorContextProcess interface {
	}

	actorContextProcessInternal interface {
		ActorContextProcess

		// getProcessId 获取当前 Actor 的 ID
		getProcessId() ActorRef

		// getProcess 获取当前 Actor 的 Process
		getProcess() Process

		// sendToProcess 向 Process 发送消息
		sendToProcess(envelope Envelope)

		// sendToSelfProcess 向当前 Actor 发送消息
		sendToSelfProcess(envelope Envelope)

		// getMailbox 获取当前 Actor 的邮箱
		getMailbox() Mailbox
	}
)

// TimingTask 是定时任务的函数类型

type (
	// ActorContextTiming 是 ActorContext 的子集，它包含了 Actor 的定时器功能
	ActorContextTiming interface {
		// After 创建一个在一段时间后发送到 Actor 邮箱中执行的任务
		After(name string, duration time.Duration, task TimingTask)

		// ForeverLoop 创建一个循环执行的任务，它将在 duration 时间后首次执行，然后根据 interval 时间再次执行
		ForeverLoop(name string, duration, interval time.Duration, task TimingTask)

		// Loop 创建一个循环执行的任务，它将在 duration 时间后首次执行，然后根据 interval 方法返回的时间再次执行，直到次数满足 times 为止
		Loop(name string, duration, interval time.Duration, times int, task TimingTask)

		// Cron 通过 cron 表达式创建一个任务，当表达式无效时将返回错误
		//  - 表达式说明可参阅：https://github.com/gorhill/cronexpr
		Cron(name string, cron string, task TimingTask) error

		// StopTimingTask 停止指定名称的任务
		StopTimingTask(name string)

		// ClearTimingTasks 停止所有任务
		ClearTimingTasks()
	}

	actorContextTimingInternal interface {
		ActorContextTiming

		getTimingWheel() timing.Named
	}
)

type (
	// ActorContextSpawner 是 ActorContext 的子集，它确保了对子 Actor 的生成
	//   - 所有的生成函数均无法保证并发安全
	ActorContextSpawner interface {
		// ActorOf 生成 Actor
		ActorOf(provider ActorProvider, configurator ...ActorConfigurator) ActorRef

		// ActorOfFn 函数式生成 Actor
		ActorOfFn(provider ActorProviderFn, configurator ...ActorConfiguratorFn) ActorRef

		// ActorOfConfig 生成 Actor
		ActorOfConfig(provider ActorProvider, config ActorConfiguration) ActorRef

		// ChainOf 通过责任链的方式生成 Actor
		ChainOf(provider ActorProvider) ActorSpawnChain
	}

	actorContextSpawnerInternal interface {
		ActorContextSpawner

		// getActor 获取当前 Actor
		getActor() Actor

		// resetActorState 重置 Actor 状态
		resetActorState()
	}
)

type (
	// ActorContextLogger 是 ActorContext 的子集，它确保了对日志的记录
	ActorContextLogger interface {
		// Logger 获取日志记录器
		Logger() log.Logger

		// GetLoggerProvider 获取日志记录器提供者
		GetLoggerProvider() log.Provider
	}

	actorContextLoggerInternal interface {
		ActorContextLogger
	}
)

type (
	// ActorContextLife 是 ActorContext 的子集，它确保了对 Actor 生命周期的控制
	ActorContextLife interface {
		// Ref 获取当前 Actor 的 ActorRef
		Ref() ActorRef
	}

	actorContextLifeInternal interface {
		ActorContextLife

		getSystemConfig() ActorSystemOptionsFetcher

		getConfig() ActorOptionsFetcher

		getNextChildGuid() int64

		bindChild(child ActorRef)

		unbindChild(ref ActorRef)

		getChildren() map[Path]ActorRef
	}
)

type (
	// ActorContextExternalRelations 是 ActorContext 的子集，它确保了 Actor 的外界关系
	ActorContextExternalRelations interface {
		// Parent 获取父 Actor 的 ActorRef
		Parent() ActorRef

		// System 获取当前 Actor 所属的 ActorSystem
		System() ActorSystem
	}

	actorContextExternalRelationsInternal interface {
		ActorContextExternalRelations

		// getMessageBuilder 获取消息构建器
		getMessageBuilder() RemoteMessageBuilder
	}
)

type (
	// ActorContextActions 是 ActorContext 的子集，它定义了 Actor 所支持的动作
	ActorContextActions interface {
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
		Watch(target ActorRef, handlers ...WatchHandler) error

		// Unwatch 取消监视目标 Actor 的生命周期
		Unwatch(target ActorRef)

		// Restart 重启目标 Actor
		Restart(target ActorRef, gracefully bool, reason ...string)
	}

	actorContextActionsInternal interface {
		ActorContextActions

		// tell 该函数用于向特定目标发送标准的消息，消息将经过包装并投递到目标 Actor 的邮箱中
		//   - 该函数在对自身发送消息时会加速投递，避免通过进程管理器进行查找
		tell(target ActorRef, message Message, messageType MessageType)

		// ask 向目标 Actor 发送消息，并返回一个 Future 用于获取结果。
		//  - 如果 timeout 参数不存在，那么将会在 DefaultFutureTimeout 时间内等待结果。
		ask(target ActorRef, message Message, messageType MessageType, timeout ...time.Duration) Future[Message]

		addWatcher(watcher ActorRef)

		deleteWatcher(watcher ActorRef)

		getWatchers() map[ActorRef]struct{}

		getWatcherHandlers(watcher ActorRef) ([]WatchHandler, bool)

		deleteWatcherHandlers(watcher ActorRef)
	}
)

type (
	// ActorContextTransport 是 ActorContext 的子集，它确保了对 Actor 之间的通信
	ActorContextTransport interface {
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

	actorContextTransportInternal interface {
		ActorContextTransport

		// setEnvelope 设置当前消息
		setEnvelope(envelope Envelope)

		// getEnvelope 获取当前消息
		getEnvelope() Envelope
	}
)

func newActorContext(system ActorSystem, config ActorOptionsFetcher, provider ActorProvider, mailbox Mailbox, ref ActorRef, parentRef ActorRef) *actorContext {
	ctx := &actorContext{}
	ctx.recipient = newActorContextRecipient(ctx)
	ctx.actorContextProcessInternal = newActorContextProcess(ctx, ref, mailbox)
	ctx.actorContextTransportInternal = newActorContextTransportImpl(ctx)
	ctx.actorContextActionsInternal = newActorContextActionsImpl(ctx)
	ctx.actorContextExternalRelationsInternal = newActorContextExternalRelationsImpl(system, ctx, parentRef)
	ctx.actorContextLifeInternal = newActorContextLifeImpl(ctx, config)
	ctx.actorContextLoggerInternal = newActorContextLoggerImpl(ctx)
	ctx.actorContextSpawnerInternal = newActorContextSpawnerImpl(ctx, provider)
	ctx.actorContextTimingInternal = newActorContextTimingImpl(ctx)
	return ctx
}

type actorContext struct {
	actorContextProcessInternal
	actorContextTransportInternal
	actorContextActionsInternal
	actorContextExternalRelationsInternal
	actorContextLifeInternal
	actorContextLoggerInternal
	actorContextSpawnerInternal
	actorContextTimingInternal

	recipient Recipient
}
