package vivid

import (
	"time"

	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/sugar"
)

// ActorContext 定义了 Actor 内部行为逻辑和运行环境的上下文接口，
// 它在每次消息处理（无论是业务消息还是系统消息）时动态生成，
// 并作为参数注入到 Actor 的行为函数（Behavior）之中，用于支撑 Actor 的核心运行能力、子 Actor 管理等。
// 所有操作均线程隔离，建议仅在本消息处理流程内使用。
type ActorContext interface {
	actorBasic
	actorRace

	// System 返回当前 ActorContext 所属的 ActorSystem 实例，即归属的顶级 Actor 系统入口。
	//
	// 使用说明：
	//   - 可通过该实例访问 Actor 系统级别配置（如全局默认 ask 超时）、顶级 Actor 资源、注册服务等功能。
	System() ActorSystem

	// EventStream 返回事件流实例。
	EventStream() EventStream

	// Message 返回当前 ActorContext 正在处理的消息实例。
	//
	// 特殊消息说明：
	// 除了业务自定义消息外，Actor 系统会自动分发若干特殊的生命周期或来自 Vivid 的特定事件消息类型。
	// 主要包括（详见 vivid/message.go）：
	//	- *vivid.OnLaunch: Actor 启动时的第一条消息，可用于初始化场景。
	//	- *vivid.OnKill: 当 Actor 自身被销毁请求时收到，可用于优雅停机处理。
	//	- *vivid.OnKilled: Actor 已终止后，通知相关方做资源收尾，如清理子 Actor 等。
	//
	// 可通过类型断言判断和区分消息种类，设计定制化的生命周期行为。
	Message() Message

	// Parent 返回当前 Actor 的父级 ActorRef。
	Parent() ActorRef

	// Children 返回当前 Actor 的所有子 ActorRefs。
	Children() ActorRefs

	// Ref 返回当前 Actor 的 ActorRef 实例。
	Ref() ActorRef

	// Sender 返回本条消息的发送者（ActorRef）。如果消息来源于系统（如 OnLaunch），则可能返回 nil。
	//
	// 使用案例：
	//   - 可用于回复请求方、追溯消息链路、权限校验等场景。
	Sender() ActorRef

	// Reply 向当前消息的发送者（即 Sender）发送回复消息（通常用于请求-响应模式）。
	//
	// 参数:
	//   - message: 希望回复发送者的消息内容（自定义或内置类型）
	//
	// 注意事项：
	//   - 若 Sender() 返回 nil（如系统消息场景），Reply 操作将被安全忽略。
	//   - 推荐仅在处理“请求-应答”业务时使用，否则无须显式回复。
	Reply(message Message)

	// Become 方法用于切换当前 Actor 的消息处理行为（Behavior）。
	//
	// 功能说明：
	//   - 该方法将新的行为函数 behavior 推入行为堆栈（行为栈顶），替换当前消息处理逻辑，实现 Actor 行为的动态变更。
	//   - 行为栈的设计允许嵌套、递归叠加新行为，可支持有限状态机、流程迁移、角色切换等丰富业务场景。
	//   - 此变更在调用后立即生效，后续收到的消息将按照新的 behavior 进行处理，直到再次切换或手动恢复。
	//
	// 参数说明：
	//   - behavior: 新的消息处理函数，类型为 Behavior，该函数定义了 Actor 收到消息时的处理逻辑。
	//   - options: 行为切换配置项（可选），用于定制行为栈的变更策略，如是否丢弃历史行为（参考 WithBehaviorDiscardOld、WithBehaviorOptions 等）。
	//
	// 使用建议：
	//   - 适用于需实现 Actor 内部状态流转、阶段性业务流程、会话等动态事件的业务场景。
	//   - 配合 UnBecome 可实现“行为回退”或多级“状态恢复”，增强 Actor 的灵活性与可测试性。
	//   - 若需覆盖所有历史行为（清空行为栈，仅保留当前新行为），可传递对应 option 参数。
	//
	// 示例用法：
	//   ctx.Become(waitForResponseBehavior)                 // 普通栈式行为切换
	//   ctx.Become(failedState, WithBehaviorDiscardOld(true)) // 切换且丢弃原有行为，仅保留新行为
	Become(behavior Behavior, options ...BehaviorOption)

	// UnBecome 方法用于恢复 Actor 先前的行为（Behavior）。
	//
	// 功能说明：
	//   - 该方法会弹出行为栈栈顶的 Behavior，恢复为先前的行为函数，通常用于“流程回退”或“状态机回溯”。
	//   - 行为栈底始终为 Actor 的初始行为（创建时注册），若已堆栈到初始行为时再次调用，则无操作。
	//   - 可配合 options 参数，实现定制的恢复策略（自定义行为栈管理）。
	//
	// 参数说明：
	//   - options: 变更恢复过程的行为参数（可选），一般用于自定义行为恢复时的策略控制。
	//
	// 使用建议：
	//   - 当任务流程需多级嵌套行为切换，阶段性完成后可调用 UnBecome 恢复先前逻辑，增强系统可读性与可维护性。
	//   - 应确保行为切换与恢复的配对关系，避免异常栈状态。
	//
	// 示例用法：
	//   ctx.UnBecome()                          // 常规行为回退
	//   ctx.UnBecome(WithBehaviorOption(...))   // 搭配自定义恢复策略
	UnBecome(options ...BehaviorOption)

	// TellSelf 向当前 ActorContext 发送消息，通常用于在当前 ActorContext 内部进行消息传递或促进事件循环。
	// 该函数等同于 Tell(ctx.Ref(), message) 的快捷方式。
	//
	// 由于是投递给自己，无需再次寻找邮箱，因此性能优于 Tell(ctx.Ref(), message)。
	TellSelf(message Message)

	// Name 返回当前 Actor 的名称。
	Name() string

	// Failed 报告当前 Actor 发生故障，将根据父级 Actor 的监督策略决定如何处理故障。
	//
	// 参数:
	//   - fault: 故障消息
	Failed(fault Message)

	// Watch 方法用于订阅并监听指定 ActorRef 的终止事件（OnKilled 消息），实现 Actor 间的死亡通知机制。
	//
	// 功能说明：
	//   - 调用本方法后，当前 ActorContext（调用者）将自动接收该被监听 Actor 在终止（Killed）时发送的 OnKilled 消息，用于感知下游或关联 Actor 生命周期结束。
	//   - 支持多级、跨节点 Actor 间的生命周期解耦，常用于聚合、持久化、依赖安全等场景，实现典型成长性高可用架构所需的“死亡监听”能力。
	//   - 每个 ActorContext 仅会注册一次监听，重复调用无副作用（幂等），无需担心多次监听导致冗余。
	//
	// 参数说明：
	//   - ref: 目标被监听的 ActorRef 实例。该引用须为有效、可用的 Actor 实例引用。
	//
	// 典型应用场景：
	//   - 高级聚合/调度 Actor 感知动态工作组成员终止行为，实现弹性扩展或错误恢复；
	//   - 基于事件风格的处理管道，解耦业务 Actor 间的直接依赖，提高可扩展性。
	//
	// 注意事项：
	//   - 仅当被监听的 ActorRef 终止（Killed）时，监听方 Actor 会收到 OnKilled 消息。
	//   - 若目标正在终止过程中收到，且已终止，将会收到最后一次死亡通知。
	//   - 若目标不存在，将会被记入死信队列。
	//   - 父级 Actor 会自动监听子 Actor，无需手动监听，自身也会默认监听自身的死亡事件。
	Watch(ref ActorRef)

	// Unwatch 方法用于取消对指定 ActorRef 的终止事件监听。
	//
	// 功能说明：
	//   - 调用本方法后，当前 ActorContext（调用者）将不再接收被取消监听的目标 ActorRef 的 OnKilled 消息。
	//   - 若未曾监听或目标已被终止，调用无副作用，接口自动忽略未注册监听的引用，保证高可用和实现幂等性。
	//   - 若目标不存在，将会被记入死信队列。
	//
	// 参数说明：
	//   - ref: 需取消监听的 ActorRef 实例。为已注册监听或计划移除的目标。
	//
	// 典型应用场景：
	//   - 管理 Actor 生命周期队列，避免发生目标重启、迁移等场景时收到冗余的死亡通知；
	//   - 需要灵活控制死亡关联关系时，便于构建复杂业务流程。
	//
	// 注意事项：
	//   - 取消后需根据业务实际做对应资源回收或状态调整，防止消息处理遗漏。
	Unwatch(ref ActorRef)

	// Stash 方法用于将当前收到的消息（Message）暂存到 ActorContext 的暂存区（stash）中，并跳过该消息的当前处理流程。
	//
	// 功能说明：
	//   - 可临时存储当前消息至 stash 区（消息队列），便于因等待条件/状态不足异常、不易立即处理的消息后续再恢复处理。
	//   - 适用于 Actor 需先处理完某些特定消息、流程“卡点”，或业务异步加载过程中主动搁置后续消息的典型场景。
	//
	// 使用建议：
	//   - 一般配合 Unstash 方法成对使用，实现消息重试、任务排队、流程限流等高阶队列管理能力；
	//   - 应结合实际业务防止消息死锁（即消息被过度 stash 导致永远无法恢复）。
	//
	// 注意事项：
	//   - stash 区与主消息处理队列隔离，合理栈管理可提升系统弹性和可恢复性。
	Stash()

	// Unstash 方法用于将 stash（临时消息暂存区）中的消息恢复到主消息队列，并开始正常分派处理。
	//
	// 功能说明：
	//   - 可指定恢复数量 num，自 stash 队列最早被暂存的消息起，按时间顺序依次投递回主队列，确保消息顺序性与安全性。
	//   - num  <= 0 时，表示恢复 stash 中的全部消息，适用于全量恢复处理场景，默认仅恢复最旧的一条消息。
	//
	// 参数说明：
	//   - num: （可选参数）需恢复的消息数量，默认仅恢复最旧的一条消息；num > 0 时仅恢复指定数量。如果 num 大于 stash 中的消息数量，则恢复所有消息。
	//
	// 典型应用场景：
	//   - Actor 处理业务流程中遇到依赖资源未到位等场景，stash 提前到达的消息，待依赖就绪通过 Unstash 一次性恢复批量处理；
	//   - 实现自定义消息限流、后置次序、调度策略等复杂队列能力。
	//
	// 注意事项：
	//   - 恢复过程中需确认 Actor 已具备完成对应消息处理的所有条件与依赖，避免反复 stash/un-stash 死循环。
	Unstash(num ...int)

	// StashCount 返回 stash 中的消息数量。
	StashCount() int
}

type actorRace interface {
	// ActorOf 在当前 ActorContext 作用域下创建一个子 Actor，并返回新子 Actor 的 ActorRef。
	//
	// 参数:
	//   - actor: 待创建的子 Actor 实例（必须实现 Actor 接口）
	//   - options: 额外配置选项，可变参数（如显式命名、邮箱容量、启动参数等）
	//
	// 返回值:
	//   - *sugar.Result[ActorRef]：子 Actor 的引用及创建过程中的异常（若有）
	//
	// 核心说明：
	//   - 仅允许父级 ActorContext 为自己的子 Actor 创建生命周期管理，保证结构树的隔离与一致性。
	//   - 该方法非并发安全，不支持多协程并发创建同级 Actor；一般用于串行业务流程。
	//   - 异常场景会返回 sugar 错误（如重名、系统资源限制等），调用方应处理创建失败的可能。
	ActorOf(actor Actor, options ...ActorOption) *sugar.Result[ActorRef]
}

// actorBasic 抽象出 Actor 基础消息操作、父节点引用与通信能力，为 ActorContext 和 ActorSystem 内部复用。
// 不建议业务方直接实现或调用，建议通过 ActorContext 间接获得各项能力。
type actorBasic interface {
	ActorLiaison

	// Kill 请求当前 Actor 终止运行，支持优雅停机（poison=false）或立即销毁（poison=true）。
	//
	// 参数:
	//   - ref: 要终止的 Actor 的引用（ActorRef）
	//   - poison: 是否采用毒杀模式，true 时立即销毁，不处理剩余队列，false 时常规优雅下线。
	//   - reason: 终止原因描述，便于追踪和日志分析，多个参数时会拼接成一个字符串（使用 ", " 分隔）。
	Kill(ref ActorRef, poison bool, reason ...string)

	// Logger 方法用于获取当前 ActorContext 的日志记录器（log.Logger）。
	//
	// 功能说明：
	//   - 当业务或框架代码需进行日志输出（如 Info、Warn、Err 日志、调试信息等）时，应通过调用本方法获取日志记录器。
	//   - 推荐统一通过本接口进行日志处理，避免直接持有底层 Logger 实例以保障日志策略的灵活变更与隔离。
	//
	// 返回策略：
	//   - 若当前 ActorContext 已显式指定专用日志记录器（通常通过 ActorOption、系统初始化参数等设置），则优先返回该日志记录器，用于实现 Actor 级别的隔离、定向输出。
	//   - 若未配置专用日志记录器，则自动回退为 ActorSystem 的全局日志记录器，实现默认共享和整体可观测性。
	//   - 任一场景下均保证返回非 nil 的 log.Logger 实例，调用方无需判空，直接调用各类日志方法；如均未设置，则返回系统内置的默认日志实现。
	//
	// 典型应用场景：
	//   - 业务 Actor 在消息处理（Handle/Behavior）过程中，需记录业务事件、追踪上下文或输出关键日志；
	//   - 框架内对 Actor 生命周期、异常、调度流程进行监控和埋点；
	//   - 支持多级日志隔离（系统级、Actor级），便于定位问题与动态调整日志策略。
	//
	// 返回值：
	//   - log.Logger：当前上下文（ActorContext 或 ActorSystem）可用的日志记录器实例。
	Logger() log.Logger

	// Metrics 方法用于获取当前 ActorContext 可用的指标收集器（metrics.Metrics）。
	//
	// 功能说明：
	//   - 支持业务 Actor 上报各类自定义、系统级的指标数据（如计数器、仪表盘、直方图等）用于运行监控与性能分析。
	//   - 使用方式请参考 metrics.Metrics 接口，建议在消息处理等流程中按需调用采集/递增指标。
	//
	// 返回策略：
	//   - 优先返回当前 ActorSystem 的指标收集器实例，实现统一采集及快照导出。
	//   - 若当前 ActorSystem 未启用任何指标采集器（即未配置 metrics.Metrics），则本方法将返回一个仅临时有效的一次性指标收集器（其状态不会被全局保存，数据无法全局使用，仅保障业务代码调用不 panic）。
	//   - 此场景下系统会自动输出一条告警日志，提示当前 ActorSystem 未启用正式指标组件，建议运维或开发人员及时补充配置以实现完整指标观测能力。
	//   - 返回值无论如何保证不为 nil，调用方无需判空，可直接完成指标埋点；但如返回为临时收集器，其功能和可见性将受限。
	//
	// 警告：
	//   - 如果你在未启用（未配置）全局指标采集时调用本方法，采集到的数据不会被纳入集中观察与管理，建议只在开发或测试场景下采用。
	Metrics() metrics.Metrics
}

type ActorLiaison interface {
	// Tell 向指定 ActorRef 异步发送消息（单向），即 Fire-and-Forget，不关心对方回复。
	//
	// 参数:
	//   - recipient: 目标 Actor 的引用（ActorRef）
	//   - message: 发送内容（任何类型，推荐结构体以增强类型安全）
	//
	// 行为特性：
	//   - 消息异步派发进入目标 Actor 的邮箱，由其所在调度器排队处理。
	//   - 永不阻塞本地调用方；不保证投递顺序（但同一发送方顺序一致）。
	Tell(recipient ActorRef, message Message)

	// Ask 向指定 ActorRef 发送请求型消息，并获得 Future 以便异步等待回复。
	//
	// 参数:
	//   - recipient: 目标 Actor 的引用（ActorRef）
	//   - message: 请求内容（任何类型，通常为业务结构体或系统事件）
	//   - timeout（可选）: 单次请求超时设定（不传则采用系统默认 ask 超时时间）
	//
	// 返回值:
	//   - Future[Message]: 表示异步应答的 Future 实例对象，可链式处理/同步等待
	//
	// 行为特性：
	//   - 支持多种超时控制与异常捕捉，超时后 Future 状态自动为失败。
	//   - 适用于 RPC、协作、需结果确认等双向通信场景。
	Ask(recipient ActorRef, message Message, timeout ...time.Duration) Future[Message]

	PipeTo(recipient ActorRef, message Message, forwarders ActorRefs, timeout ...time.Duration) string

	// Logger 返回日志记录器。
	Logger() log.Logger
}

type PrelaunchContext interface {
	// Logger 返回日志记录器。
	Logger() log.Logger

	// EventStream 返回事件流实例。
	EventStream() EventStream

	// Ref 返回当前 ActorContext 的 ActorRef。
	Ref() ActorRef
}

type RestartContext interface {
	// Logger 返回日志记录器。
	Logger() log.Logger
}
