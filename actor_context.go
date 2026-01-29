package vivid

import (
	"time"

	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/metrics"
)

// EntrustTask 定义了可在 ActorContext 中托管执行的异步任务接口。
// 实现该接口的类型可以封装并提交任意包含业务逻辑的异步任务，
// 其核心方法 Run 应返回预期结果的 Future，可用于异步等待与回调处理。
type EntrustTask interface {
	// Run 方法负责异步执行核心业务任务，并返回结果。
	Run() (Message, error)
}

// EntrustTaskFN 是对 EntrustTask 的函数式适配器，
// 使得任意满足签名 func() (Message, error) 的函数均可作为 EntrustTask 使用。
type EntrustTaskFN func() (Message, error)

// Run 会直接调用底层方法体，执行具体异步任务，并返回处理结果。
func (fn EntrustTaskFN) Run() (Message, error) {
	return fn()
}

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

	// EventStream 方法用于获取当前 ActorContext 关联的事件流（EventStream）实例。
	//
	// 功能与用途说明：
	//   - 事件流是 ActorSystem 的发布/订阅（Pub/Sub）基础设施，支持 Actor、外部组件等向系统内广播各种事件（如生命周期、业务通知等）。
	//   - Actor 可通过事件流进行自定义事件的发布（Publish）与订阅（Subscribe），用于系统解耦、横切监控、事件驱动等场景。
	//
	// 适用场景实例：
	//   - 监听 Actor 生命周期变更（如启动、终止）。
	//   - 跨模块消息广播，减少直接依赖与耦合。
	//   - 构建基于事件的扩展点、监控与告警等功能模块。
	//
	// 推荐用法：
	//   - Publish：c.EventStream().Publish(sender, event)
	//   - Subscribe：c.EventStream().Subscribe(eventType, handlerFunc)
	//
	// 返回值：
	//   - EventStream：系统唯一的事件流对象，可用于事件发布与订阅管理。
	EventStream() EventStream

	// Scheduler 方法用于获取当前 ActorContext 独享的调度器（Scheduler）实例。
	//
	// 功能与用途说明：
	//   - Scheduler 支持定时（Once）、周期性（Loop）、Cron 表达式等多种类型的异步消息调度与定时任务投递。
	//   - 所有调度任务的回调投递均自动封装为 Actor 消息，确保线程安全、串行化处理，并支持伴随 Actor 生命周期自动清理。
	//
	// 典型应用场景：
	//   - 定时发送心跳、延迟执行操作、定制定时业务逻辑。
	//   - 复杂时间控制，如基于 Cron 的计划任务等。
	//   - Actor 层自主管理子任务、定时触发自恢复/监控等。
	//
	// 推荐用法：
	//   - 一次性延时投递：c.Scheduler().Once(c.Ref(), time.Second, MyMessage{})
	//   - 周期性发送：c.Scheduler().Loop(c.Ref(), 10*time.Second, HealthCheck{})
	//   - Cron 调度：c.Scheduler().Cron(c.Ref(), "0 0 * * *", DoSomething{})
	//   - 取消定时任务：c.Scheduler().Cancel(reference)
	//   - 全部取消：c.Scheduler().CancelAll()
	//
	// 返回值：
	//   - Scheduler：面向当前 ActorContext 生命周期、隔离的调度器实例。
	Scheduler() Scheduler

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
	//
	// 功能说明：
	//   - 返回当前 Actor 在 Actor 层级树中的直接父级引用，用于访问父级 Actor 的能力（如 Tell、Ask 等）。
	//   - 若当前 Actor 为系统根 Actor（顶级 Actor），则返回 nil。
	//
	// 典型应用场景：
	//   - 向父级 Actor 发送状态报告、错误通知等消息。
	//   - 在子 Actor 中访问父级 Actor 的资源或能力。
	//   - 实现父子 Actor 间的协作与通信。
	//
	// 返回值：
	//   - ActorRef：父级 Actor 的引用，若为根 Actor 则返回 nil。
	Parent() ActorRef

	// Children 返回当前 Actor 的所有子 ActorRefs。
	//
	// 功能说明：
	//   - 返回当前 Actor 直接创建的所有子 Actor 的引用集合（ActorRefs），用于批量管理、监控或操作子 Actor。
	//   - 返回的集合为只读快照，不会随子 Actor 的创建或销毁而自动更新，需在需要时重新调用获取最新状态。
	//
	// 典型应用场景：
	//   - 批量向所有子 Actor 发送消息（如广播、同步状态等）。
	//   - 监控子 Actor 数量，实现动态负载均衡或资源管理。
	//   - 在 Actor 终止前，批量清理或通知所有子 Actor。
	//
	// 返回值：
	//   - ActorRefs：当前 Actor 的所有直接子 Actor 引用集合，若没有子 Actor 则返回空集合。
	Children() ActorRefs

	// Ref 返回当前 Actor 的 ActorRef 实例。
	//
	// 功能说明：
	//   - 返回当前 ActorContext 所属 Actor 的唯一引用标识，用于向自身或其他 Actor 传递自身引用。
	//   - ActorRef 是 Actor 的唯一标识符，可用于消息发送（Tell、Ask）、监听（Watch）等操作。
	//
	// 典型应用场景：
	//   - 向其他 Actor 传递自身引用，建立双向通信关系。
	//   - 在调度器（Scheduler）中向自身发送定时消息：c.Scheduler().Once(c.Ref(), time.Second, MyMessage{})。
	//   - 在子 Actor 创建时，将父级引用传递给子 Actor。
	//
	// 返回值：
	//   - ActorRef：当前 Actor 的引用实例，保证非 nil。
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

	// TellSelf 向当前 Actor 自身发送消息，通常用于在当前 ActorContext 内部进行消息传递或促进事件循环。
	//
	// 功能说明：
	//   - 该方法等同于 Tell(ctx.Ref(), message) 的快捷方式，用于向当前 Actor 自身发送消息。
	//   - 由于是投递给自己，无需再次查找邮箱，因此性能优于 Tell(ctx.Ref(), message)。
	//   - 消息会被异步投递到当前 Actor 的消息队列中，按照消息处理顺序依次处理。
	//
	// 典型应用场景：
	//   - 在当前消息处理完成后，触发后续处理逻辑（如状态机转换后的首次消息处理）。
	//   - 实现 Actor 内部的异步任务分解，将复杂任务拆分为多个消息逐步处理。
	//   - 在条件满足时，通过向自身发送消息来触发下一阶段的处理流程。
	//
	// 参数说明：
	//   - message: 要发送给自身的消息内容（任何类型，推荐结构体以增强类型安全）。
	//
	// 注意事项：
	//   - 消息是异步投递的，不会立即处理，会在当前消息处理完成后按顺序处理。
	//   - 应避免在消息处理中无限循环地向自身发送消息，可能导致消息队列堆积。
	TellSelf(message Message)

	// Name 返回当前 Actor 的名称。
	//
	// 功能说明：
	//   - 返回当前 Actor 的唯一名称标识，该名称在父级 Actor 作用域内必须唯一。
	//   - Actor 名称可用于日志记录、监控追踪、调试定位等场景，提升系统的可观测性。
	//
	// 命名规则：
	//   - 若创建时未显式指定名称，系统将自动生成唯一名称（通常为递增序号或 UUID）。
	//   - 名称在父级 Actor 作用域内必须唯一，重复名称会导致创建失败。
	//
	// 典型应用场景：
	//   - 日志输出时标识消息来源 Actor，便于问题追踪。
	//   - 监控系统中记录 Actor 标识，实现细粒度性能分析。
	//   - 调试时通过名称快速定位特定 Actor 实例。
	//
	// 返回值：
	//   - string：当前 Actor 的名称，保证非空。
	Name() string

	// Failed 报告当前 Actor 发生故障，将根据父级 Actor 的监督策略决定如何处理故障。
	//
	// 功能说明：
	//   - 当 Actor 在处理消息过程中遇到无法恢复的业务异常或错误时，可调用本方法主动上报故障。
	//   - 故障上报后，系统会根据父级 Actor 的监督策略（Supervision Strategy）决定处理方式，如重启、停止、升级等。
	//   - 这是 Actor 模型中的核心容错机制，允许系统在部分组件失败时仍能保持整体稳定性。
	//
	// 参数说明：
	//   - fault: 故障消息（Message），用于描述故障原因、上下文信息等，便于父级 Actor 做出合适的处理决策。
	//
	// 典型应用场景：
	//   - 业务逻辑处理失败，需要父级 Actor 介入处理或重启。
	//   - 资源获取失败（如数据库连接、外部服务不可用等），需要触发恢复机制。
	//   - 数据校验失败，需要上报异常并等待处理。
	//
	// 注意事项：
	//   - 调用本方法后，当前 Actor 的处理流程可能会被中断，父级 Actor 将根据监督策略决定后续行为。
	//   - 建议仅在确实无法在当前 Actor 内部恢复的严重错误时使用，轻微错误可通过日志记录或消息重试等方式处理。
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

	// StashCount 返回 stash（临时消息暂存区）中当前暂存的消息数量。
	//
	// 功能说明：
	//   - 返回当前 ActorContext 的 stash 区中暂存的消息总数，用于监控暂存状态、判断是否需要进行恢复处理等。
	//   - 该数量会随着 Stash() 调用而增加，随着 Unstash() 调用而减少。
	//
	// 典型应用场景：
	//   - 在业务逻辑中判断是否已积累足够多的暂存消息，决定是否触发批量恢复处理。
	//   - 监控 Actor 的消息暂存状态，用于性能分析或告警（如暂存消息过多可能表示处理瓶颈）。
	//   - 在条件满足时，根据暂存数量决定恢复策略（如全部恢复或分批恢复）。
	//
	// 返回值：
	//   - int：stash 区中当前暂存的消息数量，若没有暂存消息则返回 0。
	StashCount() int
}

// actorRace 抽象出 Actor 的子 Actor 创建能力，为 ActorContext 内部复用。
//
// 功能说明：
//   - 该接口定义了在 ActorContext 作用域下创建子 Actor 的能力，用于构建 Actor 层级树。
//   - 不建议业务方直接实现或调用，建议通过 ActorContext 间接获得各项能力。
//   - 该接口的方法非并发安全，不支持多协程并发创建同级 Actor。
type actorRace interface {
	// ActorOf 在当前 ActorContext 作用域下创建一个子 Actor，并返回新子 Actor 的 ActorRef。
	//
	// 功能说明：
	//   - 在当前 Actor 的层级树中创建一个子 Actor，子 Actor 的生命周期由当前 Actor 管理。
	//   - 子 Actor 创建后会立即启动，并收到 OnLaunch 消息进行初始化。
	//   - 父级 Actor 会自动监听子 Actor 的终止事件，无需手动调用 Watch。
	//   - 仅允许父级 ActorContext 为自己的子 Actor 创建生命周期管理，保证结构树的隔离与一致性。
	//
	// 参数说明：
	//   - actor: 待创建的子 Actor 实例（必须实现 Actor 接口），包含 Actor 的行为逻辑和初始化代码。
	//   - options: 额外配置选项（可变参数），可用于定制子 Actor 的行为，如显式命名、邮箱容量、启动参数、日志记录器等。
	//
	// 返回值：
	//   - ActorRef：成功创建的子 Actor 的引用，可用于后续的消息发送、监听等操作。
	//   - error：创建过程中的异常（若有），常见错误包括重名、系统资源限制、配置无效等。
	//
	// 典型应用场景：
	//   - 在业务逻辑中动态创建工作 Actor，实现任务分发和并行处理。
	//   - 构建 Actor 层级树，实现模块化、可管理的系统架构。
	//   - 根据业务需求动态扩展 Actor 实例，实现弹性伸缩。
	//
	// 注意事项：
	//   - 该方法非并发安全，不支持多协程并发创建同级 Actor，一般用于串行业务流程。
	//   - 异常场景会返回 error（如重名、系统资源限制等），调用方应处理创建失败的可能。
	//   - 子 Actor 的名称在父级作用域内必须唯一，重复名称会导致创建失败。
	//   - 若未显式指定名称，系统会自动生成唯一名称（通常为递增序号或 UUID）。
	ActorOf(actor Actor, options ...ActorOption) (ActorRef, error)
}

// actorBasic 抽象出 Actor 基础消息操作、父节点引用与通信能力，为 ActorContext 和 ActorSystem 内部复用。
// 不建议业务方直接实现或调用，建议通过 ActorContext 间接获得各项能力。
type actorBasic interface {
	ActorLiaison

	// Kill 请求指定 Actor 终止运行，支持优雅停机（poison=false）或立即销毁（poison=true）两种模式。
	//
	// 功能说明：
	//   - 向目标 Actor 发送终止请求，根据 poison 参数决定终止策略。
	//   - 优雅停机模式（poison=false）：Actor 会处理完当前消息和消息队列中的剩余消息，然后正常终止，适用于需要保证数据一致性的场景。
	//   - 毒杀模式（poison=true）：Actor 会立即终止，不处理剩余消息队列，适用于紧急停止或资源回收场景。
	//   - 终止请求是异步的，调用后立即返回，不会等待 Actor 实际终止完成。
	//
	// 参数说明：
	//   - ref: 要终止的 Actor 的引用（ActorRef），必须为有效的 Actor 引用。
	//   - poison: 是否采用毒杀模式，true 时立即销毁且不处理剩余队列，false 时常规优雅下线（处理完剩余消息后终止）。
	//   - reason: 终止原因描述（可变参数），便于追踪和日志分析，多个参数时会使用 ", " 拼接成一个字符串。
	//
	// 典型应用场景：
	//   - 在父级 Actor 中清理不再需要的子 Actor。
	//   - 系统关闭时批量终止所有 Actor。
	//   - 根据业务逻辑动态管理 Actor 生命周期。
	//
	// 注意事项：
	//   - 终止操作是异步的，调用后不会阻塞，Actor 的实际终止时间取决于消息处理速度和终止模式。
	//   - 优雅停机模式下，若消息队列中有大量消息，终止可能需要较长时间。
	//   - 毒杀模式下，未处理的消息可能会丢失，需根据业务需求谨慎选择。
	//   - 终止后的 Actor 会发送 OnKilled 消息给所有监听者（通过 Watch 注册的 Actor）。
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

// ActorLiaison 定义了 Actor 间的消息通信接口，提供了 Tell、Ask、Entrust 等核心消息传递能力。
//
// 功能说明：
//   - 该接口封装了 Actor 模型中最基础的消息传递操作，包括单向消息发送（Tell）、请求-响应模式（Ask）、任务委托（Entrust）等。
//   - 实现了 ActorLiaison 接口的类型（如 ActorContext、ActorSystem）都可以进行 Actor 间的消息通信。
//   - 所有消息传递操作均为异步执行，不会阻塞调用方。
//
// 典型应用场景：
//   - ActorContext 和 ActorSystem 通过实现该接口，提供统一的消息通信能力。
//   - 业务代码通过 ActorContext 调用该接口的方法，实现 Actor 间的协作与通信。
type ActorLiaison interface {
	// Tell 向指定 ActorRef 异步发送消息（单向），即 Fire-and-Forget 模式，不关心对方回复。
	//
	// 功能说明：
	//   - 向目标 Actor 异步发送消息，消息会被投递到目标 Actor 的消息队列中，等待其调度器处理。
	//   - 这是 Actor 模型中最基础的消息传递方式，适用于通知、命令、事件等单向通信场景。
	//   - 调用后立即返回，不会等待消息被处理，也不会获取处理结果。
	//
	// 参数说明：
	//   - recipient: 目标 Actor 的引用（ActorRef），必须为有效的 Actor 引用。
	//   - message: 发送内容（任何类型，推荐使用结构体以增强类型安全），消息会被序列化后投递。
	//
	// 行为特性：
	//   - 消息异步派发进入目标 Actor 的邮箱，由其所在调度器排队处理。
	//   - 永不阻塞本地调用方，调用后立即返回。
	//   - 不保证全局消息投递顺序，但同一发送方发送的消息顺序一致。
	//   - 若目标 Actor 不存在或已终止，消息会被发送到死信队列。
	//
	// 典型应用场景：
	//   - 发送通知、命令、事件等单向消息。
	//   - 实现发布-订阅模式中的消息发布。
	//   - 触发其他 Actor 执行特定操作，无需等待结果。
	//
	// 注意事项：
	//   - 若需要获取处理结果或确认消息已处理，应使用 Ask 方法。
	//   - 消息投递是尽力而为的，不保证消息一定被处理（如目标 Actor 终止）。
	Tell(recipient ActorRef, message Message)

	// Ask 向指定 ActorRef 发送请求型消息，并获得 Future 以便异步等待回复。
	//
	// 功能说明：
	//   - 向目标 Actor 发送请求消息，并返回一个 Future 对象用于异步等待和处理回复。
	//   - 这是 Actor 模型中实现请求-响应模式的核心方法，适用于需要获取处理结果的场景。
	//   - 目标 Actor 可通过 Reply 方法回复消息，回复内容会传递到 Future 中。
	//
	// 参数说明：
	//   - recipient: 目标 Actor 的引用（ActorRef），必须为有效的 Actor 引用。
	//   - message: 请求内容（任何类型，通常为业务结构体或系统事件），消息会被序列化后投递。
	//   - timeout: （可选参数）单次请求的超时设定，若不指定则采用系统默认 ask 超时时间。
	//
	// 返回值：
	//   - Future[Message]: 表示异步应答的 Future 实例对象，可通过 Result() 同步等待、OnComplete() 异步回调、或链式操作处理结果。
	//
	// 行为特性：
	//   - 支持多种超时控制与异常捕捉，超时后 Future 状态自动为失败，可通过 Future.Error() 获取超时错误。
	//   - 若目标 Actor 不存在或已终止，Future 会立即失败。
	//   - 适用于 RPC、协作、需结果确认等双向通信场景。
	//
	// 典型应用场景：
	//   - 向其他 Actor 查询数据或状态，等待返回结果。
	//   - 实现 RPC 风格的远程调用，获取处理结果。
	//   - 在业务流程中需要确认操作是否成功时使用。
	//
	// 推荐用法示例：
	//   future := ctx.Ask(targetActor, &QueryRequest{ID: 123}, 5*time.Second)
	//   result, err := future.Result()
	//   if err != nil {
	//       // 处理错误或超时
	//   }
	//
	// 注意事项：
	//   - 目标 Actor 必须通过 Reply 方法回复消息，否则 Future 会一直等待直到超时。
	//   - 超时时间应根据业务需求合理设置，避免过长导致资源占用或过短导致正常请求失败。
	//   - 若目标 Actor 不存在或已终止，Future 会立即失败，调用方应处理此类异常情况。
	Ask(recipient ActorRef, message Message, timeout ...time.Duration) Future[Message]

	// Entrust 方法用于在当前 ActorContext 中安全地异步托管一个可自定义的业务任务（EntrustTask），并以 Future[Message] 形式返回其异步结果。
	//
	// 功能与行为说明：
	//   - 支持将任何实现 EntrustTask 接口的异步任务委托给 ActorContext 持有的独立执行环境；
	//   - 所有异步任务均在独立的 Goroutine 中调度运行，任务实际执行流程与 Actor 独立，不会阻塞本地业务流程；
	//   - 提供超时管理能力，timeout 参数用于指定此次任务最大等待时长（如为 0，则不设置超时限制）；
	//   - 返回值 Future[Message] 可在客户端异步等待、回调处理或超时监控，适用于任务结果依赖、链式业务处理等异步编程场景；
	//   - 任务运行期间如发生任何 panic，将由内部机制自动捕获并转换为安全的错误事件传递到 Future，确保调用方业务流程不被中断或崩溃，提升系统健壮性和易维护性；
	//
	// 推荐用法示例：
	//   future := ctx.Entrust(3*time.Second, EntrustTaskFN(func() (Message, error) {
	//       // 业务逻辑处理
	//       return &MyResult{}, nil
	//   }))
	//   result, err := future.Result()
	//
	// 参数：
	//   - timeout: time.Duration 指本次委托任务的最大执行时长（超时后 Future 自动失败，上下文安全回收）
	//   - task   : EntrustTask 需被异步调度与托管的任务对象，需实现 Run() (Message, error) 接口
	//
	// 返回：
	//   - Future[Message]: 可链式操作、等待或异步回调的任务结果，panic 安全、强隔离保障业务稳定
	Entrust(timeout time.Duration, task EntrustTask) Future[Message]

	// PipeTo 方法用于将 Ask 请求的结果（Future）自动转发给指定的目标 Actor，实现请求-响应-转发的管道式消息流转。
	//
	// 功能与行为说明：
	//   - 该方法会等待 Ask 请求的 Future 结果，并将结果消息自动转发给 recipient（目标 Actor）。
	//   - 支持配置多个转发者（forwarders），当 Future 成功时，结果会同时发送给 recipient 和所有 forwarders。
	//   - 若 Future 超时或失败，则不会进行转发，调用方可通过返回的管道标识符（pipeline ID）进行后续处理或取消。
	//   - 所有转发操作均为异步执行，不会阻塞当前消息处理流程。
	//
	// 参数说明：
	//   - recipient: 主要目标 Actor 的引用（ActorRef），Future 成功时会将结果消息发送给该 Actor。
	//   - message: 用于发起 Ask 请求的消息内容，该消息会被发送给某个 Actor 并等待其回复。
	//   - forwarders: 额外的转发目标 Actor 引用集合（ActorRefs），Future 成功时会将结果同时转发给这些 Actor。
	//   - timeout: （可选参数）Ask 请求的超时时间，若不指定则使用系统默认 ask 超时时间。
	//
	// 返回值：
	//   - string: 管道标识符（pipeline ID），可用于后续取消该管道操作（如需要）。
	//
	// 注意事项：
	//   - 若 Ask 请求超时或失败，不会触发任何转发操作，调用方需根据业务需求处理超时场景。
	//   - 转发操作是异步的，调用方不应依赖转发完成的时机。
	//   - forwarders 收到的消息为 *vivid.PipeResult 类型。
	PipeTo(recipient ActorRef, message Message, forwarders ActorRefs, timeout ...time.Duration) string

	// Logger 返回日志记录器。
	//
	// 功能说明：
	//   - 返回当前 ActorLiaison 上下文可用的日志记录器实例，用于进行日志输出。
	//   - 日志记录器的返回策略与 ActorContext.Logger() 相同，优先返回 Actor 专用日志记录器，否则返回系统全局日志记录器。
	//
	// 返回值：
	//   - log.Logger：当前上下文可用的日志记录器实例，保证非 nil。
	Logger() log.Logger
}

// PrelaunchContext 定义了 Actor 启动前（Pre-launch）阶段的上下文接口，用于在 Actor 正式启动前进行初始化配置。
//
// 功能说明：
//   - 该上下文在 Actor 创建后、正式启动（OnLaunch 消息处理）前提供，允许在 Actor 初始化阶段访问系统资源。
//   - 主要用于 Actor 的构造函数或初始化方法中，进行必要的配置、资源准备等操作。
//   - 与 ActorContext 不同，PrelaunchContext 不提供消息发送、子 Actor 创建等运行时能力，仅提供基础的系统访问接口。
//
// 典型应用场景：
//   - 在 Actor 构造函数中获取日志记录器，进行初始化日志输出。
//   - 订阅系统事件流，在 Actor 启动前注册事件监听器。
//   - 获取自身 ActorRef，用于后续初始化配置或传递给其他组件。
type PrelaunchContext interface {
	// Logger 返回日志记录器。
	//
	// 功能说明：
	//   - 返回当前 PrelaunchContext 可用的日志记录器实例，用于在 Actor 启动前进行日志输出。
	//   - 日志记录器的返回策略与 ActorContext.Logger() 相同，优先返回 Actor 专用日志记录器，否则返回系统全局日志记录器。
	//
	// 返回值：
	//   - log.Logger：当前上下文可用的日志记录器实例，保证非 nil。
	Logger() log.Logger

	// EventStream 返回事件流实例。
	//
	// 功能说明：
	//   - 返回系统的事件流（EventStream）实例，用于在 Actor 启动前订阅系统事件或发布初始化事件。
	//   - 与 ActorContext.EventStream() 返回的是同一个系统级事件流实例。
	//
	// 典型应用场景：
	//   - 在 Actor 初始化阶段订阅系统生命周期事件，如其他 Actor 的启动、终止等。
	//   - 发布 Actor 初始化完成事件，通知其他组件。
	//
	// 返回值：
	//   - EventStream：系统唯一的事件流对象，可用于事件发布与订阅管理。
	EventStream() EventStream

	// Ref 返回当前 Actor 的 ActorRef 实例。
	//
	// 功能说明：
	//   - 返回当前正在初始化的 Actor 的引用标识，用于在启动前获取自身引用。
	//
	// 典型应用场景：
	//   - 在初始化阶段将自身引用传递给其他组件或进行配置。
	//   - 在构造函数中保存自身引用，供后续使用。
	//
	// 返回值：
	//   - ActorRef：当前 Actor 的引用实例，保证非 nil。
	Ref() ActorRef
}

// RestartContext 定义了 Actor 重启（Restart）阶段的上下文接口，用于在 Actor 重启过程中进行必要的清理和恢复操作。
//
// 功能说明：
//   - 该上下文在 Actor 因故障或监督策略触发重启时提供，允许在重启过程中访问基础系统资源。
//   - 主要用于 Actor 的重启处理逻辑中，进行资源清理、状态恢复、日志记录等操作。
//   - 与 ActorContext 不同，RestartContext 仅提供基础的系统访问接口，不提供消息发送、子 Actor 管理等运行时能力。
//
// 典型应用场景：
//   - 在重启处理逻辑中记录重启原因、上下文信息等日志。
//   - 清理临时资源、重置状态，为 Actor 重新启动做准备。
//   - 根据重启原因决定是否需要进行特定的恢复操作。
type RestartContext interface {
	// Logger 返回日志记录器。
	//
	// 功能说明：
	//   - 返回当前 RestartContext 可用的日志记录器实例，用于在 Actor 重启过程中进行日志输出。
	//   - 日志记录器的返回策略与 ActorContext.Logger() 相同，优先返回 Actor 专用日志记录器，否则返回系统全局日志记录器。
	//
	// 典型应用场景：
	//   - 记录重启原因、故障信息、恢复步骤等关键日志。
	//   - 输出重启过程中的调试信息，便于问题排查。
	//
	// 返回值：
	//   - log.Logger：当前上下文可用的日志记录器实例，保证非 nil。
	Logger() log.Logger
}
