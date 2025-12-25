package vivid

import (
	"time"

	"github.com/kercylan98/vivid/pkg/log"
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

	// Become 用新的行为（Behavior）函数替换当前行为，新的行为函数会被推入行为栈顶，直至下一次切换或恢复。
	//
	// 场景说明：
	//   - 可实现 Actor 状态机、行为迁移、动态消息处理能力。
	//   - 调用后立即生效，下次收到的消息由新行为逻辑处理。
	//
	// 注意：
	//   - 行为切换为堆栈管理，可嵌套调用实现复杂流程。
	Become(behavior Behavior)

	// RevertBehavior 行为恢复，将行为栈弹出回退到上一个行为状态，并返回是否成功恢复。
	//
	// 返回：
	//   - true  : 已成功恢复到上一个行为
	//   - false : 当前行为为初始栈底，无法再退
	//
	// 说明：
	//   - 适合在阶段性流程、状态退出等场景调用。
	RevertBehavior() bool

	// TellSelf 向当前 ActorContext 发送消息，通常用于在当前 ActorContext 内部进行消息传递或促进事件循环。
	// 该函数等同于 Tell(ctx.Ref(), message) 的快捷方式。
	//
	// 由于是投递给自己，无需再次寻找邮箱，因此性能优于 Tell(ctx.Ref(), message)。
	TellSelf(message Message)

	// Name 返回当前 Actor 的名称。
	Name() string
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

	// Logger 返回日志记录器。
	Logger() log.Logger
}

type PrelaunchContext interface {
	// Logger 返回日志记录器。
	Logger() log.Logger
}
