package vivid

import (
	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/go-log/log"
	"time"
)

var (
	_ ActorContext = (*actorContext)(nil) // 确保 actorContext 实现了 ActorContext 接口
)

// ActorContext 定义了一个完整的 Actor 上下文，包含了与 Actor 运行相关的所有信息和功能。
// 它是 Actor 生命周期管理和消息处理的重要组成部分，提供了多种方法用于管理 Actor 状态、调度消息、访问上下文信息等。
//
// 使用 ActorContext 时，必须严格遵循以下注意事项：
//   - 在没有明确需求的情况下，不要将 ActorContext 传递给 Actor.OnReceive 以外的地方，
//     以避免潜在的状态泄露问题，这可能导致数据竞争（race condition）或不一致的行为。
//   - ActorContext 内的所有方法在 Actor.OnReceive 内调用时是并发安全的，确保消息处理的线程安全性。
//     然而，当在 Actor.OnReceive 之外调用时，必须格外注意并发安全问题，可能需要额外的同步机制来保证线程安全。
//   - 在多个 Actor 之间传递消息时，应当避免不必要的跨 Actor 边界的调用，
//     以确保系统的可扩展性和并发性能。
//
// 总之，ActorContext 是 Actor 系统中的关键组件，合理使用和管理 ActorContext 对于系统的稳定性、可维护性和性能至关重要。
// 需要在设计和实现中遵循并发安全原则，并尽量避免不必要的状态共享和副作用。
type ActorContext interface {
	actorContextProcessInternal
	actorContextSpawnerInternal
	actorContextLoggerInternal
	actorContextLifeInternal
	actorContextTransportInternal
	actorContextTimingInternal
	actorContextPersistentInternal
}

type (
	// ActorContextProcess 是 ActorContext 的一个子集，专注于处理与 Actor 相关的进程（Process）操作。
	// 它确保了 Actor 在进程层面上的操作逻辑，如进程的启动、停止以及相关资源的管理。
	// 使用 ActorContextProcess 时，确保仅在必要时进行进程管理操作，避免不必要的进程状态泄露。
	// 此接口的实现应保证进程操作的并发安全性，并且严格按照 Actor 生命周期进行操作。
	ActorContextProcess interface{}

	actorContextProcessInternal interface {
		ActorContextProcess

		// getProcessId 获取当前 Actor 的唯一标识符，通常用于标识该 Actor 在系统中的位置。
		//
		// 注意：此函数仅在 Actor 的上下文中有效，且返回值是 Actor 的引用（ActorRef），
		// 它将帮助系统跟踪和管理该 Actor 在整个 Actor 系统中的状态。
		getProcessId() ActorRef

		// getProcess 获取当前 Actor 对应的 Process 实例。
		getProcess() Process

		// sendToProcess 将消息投递至包装指定接收人的进程中。
		//
		// 该方法将消息包装投递到 Process 中，进而处理消息的分发和执行。
		// 如果目标位于远端，应确保 Envelope.GetMessage 可以得到支持跨网络的消息。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		sendToProcess(envelope Envelope)

		// sendToSelfProcess 将消息投递至当前 Actor 的进程中，该函数将直接进行消息的投递，而不再需要通过进程管理器查找进程信息。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		sendToSelfProcess(envelope Envelope)

		// getMailbox 获取当前 Actor 的邮箱。
		// Actor 的邮箱用于存储接收到的消息，并提供相应的接口来处理消息的投递和消费。
		getMailbox() Mailbox
	}
)

type (
	ActorContextPersistent interface {
		// Snapshot 为当前 Actor 创建持久化快照并移除历史事件。
		Snapshot(snapshot Message)

		// Persist 主动将当前 Actor 的快照和事件持久化存储。
		Persist() error
	}

	actorContextPersistentInternal interface {
		ActorContextPersistent

		// persistentRecover 恢复 Actor 的持久化状态
		persistentRecover()

		// persistentMessageParse 记录持久化事件
		persistentMessageParse(envelope Envelope) Envelope

		// isPersistentMessage 判断当前消息是否为持久化消息
		isPersistentMessage() bool

		// setPersistentMessage 设置当前消息为持久化消息
		setPersistentMessage()
	}
)

type (
	// ActorContextTiming 是 ActorContext 的一个子集，提供了与定时器相关的功能。
	// 它允许在 Actor 中设置定时任务（如定时执行某些操作、周期性事件等），
	// 并确保定时任务的调度和执行过程符合 Actor 的并发模型。
	// 在使用 ActorContextTiming 时，应避免不必要的定时任务重入或调度冲突，以保证系统的性能和稳定性。
	ActorContextTiming interface {
		// After 创建一个延迟执行的任务，该任务将在指定的时间后被发送到 Actor 的邮箱。
		//
		// 参数 `name` 为任务名称，`duration` 为延迟时间，`task` 为延迟执行的任务。
		// 该任务将在指定时间后执行，通常用于延时处理。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		//
		// 需要注意：
		//  - 需要避免在 TimingTask 中将 ActorContext 传递到外部进行使用。
		After(name string, duration time.Duration, task TimingTask)

		// ForeverLoop 创建一个循环执行的定时任务，该任务将在第一次执行后每隔一段时间继续执行。
		//
		// 参数 `name` 为任务名称，`duration` 为首次执行的延迟时间，`interval` 为循环执行的间隔时间，`task` 为要执行的任务。
		// 该任务会持续执行，直到显式停止或系统终止。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		//
		// 需要注意：
		//  - 需要避免在 TimingTask 中将 ActorContext 传递到外部进行使用。
		ForeverLoop(name string, duration, interval time.Duration, task TimingTask)

		// Loop 创建一个具有次数限制的循环任务，该任务将在指定的延迟时间后首次执行，并根据设定的间隔时间执行。
		//
		// 参数 `name` 为任务名称，`duration` 为首次执行的延迟时间，`interval` 为间隔时间，`times` 为任务执行的次数。
		// 当 times 的值为 0 时，任务将不会被执行，如果 times 的值为负数，那么任务将永远不会停止，除非主动停止
		// 该任务会在执行次数达到预定值时停止。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		//
		// 需要注意：
		//  - 需要避免在 TimingTask 中将 ActorContext 传递到外部进行使用。
		Loop(name string, duration, interval time.Duration, times int, task TimingTask)

		// Cron 创建一个基于 cron 表达式的定时任务，该任务根据给定的 cron 表达式定时执行。
		//
		// 参数 `name` 为任务名称，`cron` 为 cron 表达式，`task` 为要执行的任务。
		// 该任务的执行时间由 cron 表达式决定，可以在指定时间内多次触发执行。
		// 如果 cron 表达式无效，函数将返回错误。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		//
		// 需要注意：
		//  - 需要避免在 TimingTask 中将 ActorContext 传递到外部进行使用。
		//
		// 表达式说明可参阅：https://github.com/gorhill/cronexpr
		Cron(name string, cron string, task TimingTask) error

		// StopTimingTask 停止指定名称的定时任务。
		//
		// 该方法会停止指定名称的定时任务，并清理相关资源。使用时确保提供正确的任务名称。
		// 当名称无效时，不会发生任何反应。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		StopTimingTask(name string)

		// ClearTimingTasks 停止当前所有正在执行或尚未执行的定时任务。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		ClearTimingTasks()
	}

	actorContextTimingInternal interface {
		ActorContextTiming

		// 获取该 Actor 全局的子定时器
		getTimingWheel() timing.Named

		// accidentAfter 创建一个在一段时间后发送到 Actor 邮箱中执行的系统级事故决策任务
		accidentAfter(name string, duration time.Duration, task accidentTimingTask)
	}
)

type (
	// ActorContextSpawner 是 ActorContext 的一个子集，负责 Actor 的生成与生命周期管理。
	// 它提供了接口用于创建子 Actor 生命周期。
	ActorContextSpawner interface {
		// ActorOf 创建并返回一个新的 Actor 实例。
		//
		// 该方法通过提供者和可选的配置器生成新的 Actor，返回值为 Actor 的引用（ActorRef）。
		// 使用此方法时，确保提供的 ActorProvider 和配置器能够正确初始化 Actor。
		//
		// 该函数在多个 goroutine 中调用时无法保证并发安全。
		ActorOf(provider ActorProvider, configurator ...ActorConfigurator) ActorRef

		// ActorOfFn 使用函数式编程方式生成新的 Actor。
		//
		// 该方法接收一个 ActorProviderFn 和可选的 ActorConfiguratorFn，用于生成并返回一个新的 Actor 实例。
		// 使用函数式接口可以灵活地配置 Actor 的创建过程。
		//
		// 该函数在多个 goroutine 中调用时无法保证并发安全。
		ActorOfFn(provider ActorProviderFn, configurator ...ActorConfiguratorFn) ActorRef

		// ActorOfConfig 使用配置化方式生成 Actor。
		//
		// 参数 `config` 是一个 ActorConfiguration 实例，包含了创建 Actor 所需的配置。
		// 使用此方法时，可以根据特定的配置生成 Actor。
		//
		// 该函数在多个 goroutine 中调用时无法保证并发安全。
		ActorOfConfig(provider ActorProvider, config ActorConfiguration) ActorRef

		// ChainOf 通过责任链模式生成一个新的 Actor。
		//
		// 该方法通过 ActorProvider 创建 Actor，并将多个处理步骤链接在一起形成责任链。
		// 使用此方法时，可以灵活配置 Actor 的创建流程。
		//
		// 该函数在多个 goroutine 中调用时无法保证并发安全。
		ChainOf(provider ActorProvider) ActorSpawnChain
	}

	actorContextSpawnerInternal interface {
		ActorContextSpawner
	}
)

type (
	// ActorContextLogger 是 ActorContext 的一个子集，专注于日志记录与追踪。
	// 它提供了日志功能，允许在 Actor 的生命周期中记录重要事件、状态变更、错误信息等。
	// 使用 ActorContextLogger 时，需要合理设计日志的记录频率与内容，避免日志泛滥或漏记录关键事件。
	// 同时，应确保日志记录不影响 Actor 的性能和响应时间，且能准确反映系统的状态。
	ActorContextLogger interface {
		// Logger 获取 Actor 的日志记录器。
		//
		// 该方法返回一个日志记录器实例，可以用于在 Actor 内部进行日志记录。
		// 返回的 Logger 可用于调试、错误跟踪和系统监控。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		Logger() log.Logger
	}

	actorContextLoggerInternal interface {
		ActorContextLogger

		// getLoggerProvider 获取日志记录器提供者
		getLoggerProvider() log.Provider
	}
)

type (
	// ActorContextLife 是 ActorContext 的子集，它提供了对于 Actor 生命周期中的一些信息的访问及控制。
	ActorContextLife interface {
		// System 获取当前 Actor 所在的 ActorSystem。
		//
		// 该方法返回一个 ActorSystem 实例，表示当前 Actor 所处的系统环境，
		// 该系统负责管理 Actor 的生命周期、消息调度和资源分配。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		System() ActorSystem

		// Parent 获取当前 Actor 的父 Actor 引用。
		//
		// 该方法返回当前 Actor 的父 Actor 的引用（ActorRef），
		// 通常用于 Actor 之间的层级关系管理。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		Parent() ActorRef

		// Ref 获取当前 Actor 的引用。
		//
		// 该方法返回当前 Actor 的 ActorRef，
		// 如果通过 ActorSystem 获取该 Actor，将返回根 Actor 的引用。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		Ref() ActorRef
	}

	actorContextLifeInternal interface {
		ActorContextLife

		// getSystemConfig 获取 Actor 系统配置
		getSystemConfig() ActorSystemOptionsFetcher

		// getConfig 获取 Actor 配置
		getConfig() ActorOptionsFetcher

		// getActor 获取当前 Actor
		getActor() Actor

		// resetActorState 重置 Actor 状态
		resetActorState()

		// getNextChildGuid 获取下一个子 Actor 的 GUID
		getNextChildGuid() int64

		// bindChild 绑定子 Actor
		bindChild(child ActorRef)

		// unbindChild 解绑子 Actor
		unbindChild(ref ActorRef)

		// getChildren 获取所有子 Actor
		getChildren() map[Path]ActorRef

		// getMessageBuilder 获取消息构建器
		getMessageBuilder() RemoteMessageBuilder

		// onAccident 当 Actor 发生事故时的处理
		onAccident(reason Message)

		// removeAccidentRecord 移除事故记录
		removeAccidentRecord(removedHandler func(record AccidentRecord))

		// onKill 通过 OnKill 事件驱动终止 Actor
		onKill(event OnKill)

		// onKilled 告知该 Actor 其 Sender 已经终止
		onKilled()

		// terminated 判断 Actor 是否已经终止
		terminated() bool

		// onAccidentRecord 处理事故记录
		onAccidentRecord(record AccidentRecord)

		// onAccidentFinished 处理事故结束
		onAccidentFinished(record AccidentRecord)

		// onReceive 由 Actor 处理当前消息。
		//
		// 该方法用于处理投递到当前 Actor 的消息，消息的类型和处理逻辑由 Actor 实现。
		onReceive()

		// onReceiveEnvelope 由 Actor 处理指定的消息封装。
		//
		// 该函数会在执行完毕后将当前正在处理的 Envelope 进行恢复。
		onReceiveEnvelope(envelope Envelope)
	}
)

type (
	// ActorContextTransport 是 ActorContext 的一个子集，定义了 Actor 之间的通信接口。
	// 它负责实现 Actor 之间的消息传递、数据交换和远程调用等功能。
	// 使用 ActorContextTransport 时，必须确保消息传递的可靠性和高效性，避免数据丢失或通信阻塞。
	// 需要特别关注网络延迟和异常情况的处理，确保在并发环境下的稳定通信。
	ActorContextTransport interface {
		ActorContextTransportInteractive

		// Sender 返回当前消息的发送者（ActorRef），用于标识并与发送该消息的 Actor 进行交互。
		//
		// 注意事项：
		//  - 该函数始终返回一个有效的 ActorRef，如果当前消息没有明确的发送者，则可能返回一个匿名引用或系统默认的引用。
		//  - 该函数是并发安全的，可以安全地在多个 goroutine 中调用。
		//  - 返回的 ActorRef 只能用于消息传递，不能直接访问对方的内部状态，否则会破坏 Actor 的封装性和隔离性。
		Sender() ActorRef

		// Message 返回当前待处理的消息，调用者可以通过该方法获取具体的消息内容。
		//
		// 注意事项：
		//  - 该函数始终返回一个有效的 Message 对象，调用者需要根据消息的具体类型进行处理。
		//  - 该函数是并发安全的，可以安全地在多个 goroutine 中调用。
		Message() Message

		// Reply 向当前消息的发送者（Sender）回复一个消息。
		// 该方法是 ActorContextTransportInteractive.Tell 的快捷方式，简化了回复操作。
		//
		// 使用规则：
		//  - 该函数可以在 Actor 处理当前消息时随时调用，且可以多次回复不同的消息。
		//  - 若当前消息没有明确的发送者（例如系统消息或匿名请求），则调用该方法不会产生任何效果。
		//  - 该函数是并发安全的，可以安全地在多个 goroutine 中调用。
		//  - 如果是在跨网络通讯中，回复的消息 `message` 必须是可被 Codec 序列化的，否则可能导致传输失败或异常行为。
		//  - 避免在 Reply 之后继续使用 `message`，以防止意外的并发修改。
		Reply(message Message)
	}

	// ActorContextTransportInteractive 是 ActorContextTransport 的一个子集，专门处理 Actor 之间的交互操作。
	// 它提供了一些方法，用于与其他 Actor 进行消息传递、生命周期管理等交互操作。
	// 使用 ActorContextTransportInteractive 时，应确保交互操作的顺序性和一致性，以避免出现并发问题。
	ActorContextTransportInteractive interface {
		// Kill 忽略一切尚未处理的消息，立即终止目标 Actor。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		// 在调用时会强制终止目标 Actor，忽略所有待处理消息，这可能导致目标 Actor 内部状态的不一致，
		// 请谨慎使用，特别是在 Actor 状态不允许被中断的场景中。
		Kill(target ActorRef, reason ...string)

		// PoisonKill 等待目标 Actor 处理完当前所有消息后终止目标 Actor。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		// 与 Kill 不同，PoisonKill 会确保目标 Actor 完成当前剩余消息的处理后再终止，
		// 这种方式适用于希望目标 Actor 在终止前完成当前工作或清理的场景。
		PoisonKill(target ActorRef, reason ...string)

		// Tell 向指定的 Actor 发送消息。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		// 该方法将消息发送到目标 Actor，目标 Actor 会在其消息处理队列中接收到该消息。
		// 此方法不会等待处理结果，适合于单向的消息传递。
		Tell(target ActorRef, message Message)

		// Ask 向目标 Actor 发送消息，并返回一个 Future 用于获取结果。
		// - 如果 timeout 参数不存在，那么将会在 DefaultFutureTimeout 时间内等待结果。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		// Ask 是一个请求-响应模式的操作，发送消息后会返回一个 Future 对象，
		// 可以通过该 Future 对象等待目标 Actor 返回的响应消息，适合于需要获取结果的场景。
		Ask(target ActorRef, message Message, timeout ...time.Duration) Future[Message]

		// Broadcast 向所有子 Actor 发送消息。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		// 该方法将消息广播到所有当前 Actor 的子 Actor，适合用于群发通知或信息同步等场景。
		Broadcast(message Message)

		// Ping 尝试对目标 Actor 发送 Ping 消息，并返回 Pong 消息。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		// Ping 通过向目标 Actor 发送 Ping 消息来测试与其的连接是否正常，目标 Actor 在收到后将会返回 Pong 消息。
		// 适用于检查连接健康状态或进行通信确认。
		Ping(target ActorRef, timeout ...time.Duration) (Pong, error)

		// Watch 监视目标 Actor 的生命周期，当目标 Actor 终止时，会收到 OnWatchStopped 消息。
		// 该函数会向目标 Actor 发送 Watch 消息，目标 Actor 收到 Watch 消息后会将自己加入到监视列表中。
		// - 如果传入了 handler 函数，那么当目标 Actor 终止时，会调用 handler 函数，而不再投递 OnWatchStopped 消息。
		// - handler 的调用是在当前 Actor 中作为消息进行处理的。
		// - 如果 handler 存在多个，那么会按照顺序调用。
		// - 重复的调用会追加更多的 handler。
		//
		// 该函数在多个 goroutine 中调用时无法保证并发安全。
		// Watch 用于监控目标 Actor 的生命周期，通常用于 Actor 之间的依赖管理或资源清理。
		// 在使用时应注意 handler 的执行顺序。
		Watch(target ActorRef, handlers ...WatchHandler) error

		// Unwatch 取消监视目标 Actor 的生命周期。
		//
		// 该函数在多个 goroutine 中调用时无法保证并发安全。
		// 调用该方法将取消当前 Actor 对目标 Actor 的监视，并停止接收 OnWatchStopped 消息。
		// 如果在 Watch 中注册了 handler，那么 handler 将不再被触发。
		Unwatch(target ActorRef)

		// Restart 重启目标 Actor。
		//
		// 该函数是并发安全的，可以在多个 goroutine 中调用。
		// Restart 会重新启动目标 Actor，给定是否优雅重启的标志，
		// 优雅重启会等待目标 Actor 完成当前处理的任务后再进行重启，适用于对 Actor 状态要求较高的场景。
		Restart(target ActorRef, gracefully bool, reason ...string)
	}

	actorContextTransportInternal interface {
		ActorContextTransport

		// tell 该函数用于向特定目标发送标准的消息，消息将经过包装并投递到目标 Actor 的邮箱中
		//   - 该函数在对自身发送消息时会加速投递，避免通过进程管理器进行查找
		tell(target ActorRef, message Message, messageType MessageType)

		// ask 向目标 Actor 发送消息，并返回一个 Future 用于获取结果。
		//  - 如果 timeout 参数不存在，那么将会在 DefaultFutureTimeout 时间内等待结果。
		ask(target ActorRef, message Message, messageType MessageType, timeout ...time.Duration) Future[Message]

		// onProcessMessage 当 Actor 收到的消息到达时，通过该函数进行处理
		onProcessMessage(envelope Envelope)

		// getWatchers 获取监视者列表
		getWatchers() map[ActorRef]struct{}

		// onWatchStopped 通过 OnWatchStopped 消息告知监视者目标已经终止
		onWatchStopped(m OnWatchStopped)

		// setEnvelope 设置 Actor 当前处理的 envelope。
		//
		// 该函数用于内部使用，允许外部代码设置 Actor 当前正在处理的 envelope，
		// 请谨慎使用，以免影响 Actor 的消息处理过程。
		setEnvelope(envelope Envelope)

		// getEnvelope 获取 Actor 当前处理的 envelope。
		//
		// 该函数返回 Actor 当前正在处理的 envelope，通常用于调试或诊断 Actor 的运行状态。
		// 在实际使用中，避免过度依赖此方法，尽量通过 Actor 的消息处理流程来获取所需信息。
		getEnvelope() Envelope
	}
)

// newActorContext 创建并初始化一个新的 actorContext 实例。
// 此函数会根据传入的系统、配置、提供者、邮箱、引用及父引用等信息，构建并返回一个新的 Actor 上下文。
// 它将初始化与 Actor 生命周期、消息传递、日志、定时任务等相关的所有子系统。
//
// 使用该函数时，确保传入的各项参数都已经正确配置，
// 以避免因错误的配置导致 Actor 无法正常运行。
func newActorContext(system ActorSystem, config ActorOptionsFetcher, provider ActorProvider, mailbox Mailbox, ref ActorRef, parentRef ActorRef) *actorContext {
	ctx := &actorContext{}
	ctx.recipient = newActorContextRecipient(ctx)
	ctx.actorContextProcessInternal = newActorContextProcess(ctx, ref, mailbox)
	ctx.actorContextTransportInternal = newActorContextTransportImpl(ctx)
	ctx.actorContextLifeInternal = newActorContextLifeImpl(system, ctx, config, provider, parentRef)
	ctx.actorContextLoggerInternal = newActorContextLoggerImpl(ctx)
	ctx.actorContextSpawnerInternal = newActorContextSpawnerImpl(ctx)
	ctx.actorContextTimingInternal = newActorContextTimingImpl(ctx)
	ctx.actorContextPersistentInternal = newActorContextPersistentImpl(ctx)
	return ctx
}

type actorContext struct {
	actorContextProcessInternal
	actorContextTransportInternal
	actorContextLifeInternal
	actorContextLoggerInternal
	actorContextSpawnerInternal
	actorContextTimingInternal
	actorContextPersistentInternal

	recipient Recipient
}
