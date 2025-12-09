package vivid

import (
	"time"

	"github.com/kercylan98/vivid/pkg/log"
)

// ActorSystem 定义了 Actor 系统的核心接口，代表管理所有 Actor 的顶层实体。
//
// 主要职责：
//   - 提供 Actor 系统级别的能力（如消息发送、Actor 生命周期管理等）。
//   - 为所有 ActorContext 和 ActorRef 提供统一的系统访问入口，实现协程隔离和线程安全。
//   - 通过组合内部 actorCore 接口，继承了父引用、消息发送（Tell/Ask）等基础功能。
//
// 典型用法：
//   - 应用启动时创建唯一的 ActorSystem 实例，通过该实例衍生、管理其子 Actor。
//   - 推荐通过 NewActorSystem（见 bootstrap 包）工厂方法创建实例，并使用泛型 result.Result 进行错误处理与解包。
//
// 注意事项：
//   - ActorSystem 实例设计为轻量且线程安全，避免作为全局变量暴露在多线程环境下共享。
//   - 所有 Actor 的创建、消息调度、行为切换等应严格由所属 ActorSystem 实现和调度，保证隔离与安全。
type ActorSystem interface {
	actorBasic // 内嵌 actorCore 接口，继承 Actor 系统基础能力

	// Stop 优雅地停止当前 ActorSystem 实例。
	//
	// 主要功能与行为说明：
	//   - 调用后会触发 ActorSystem 及其全部托管的 Actor（包括根 Actor 及所有子 Actor）的有序关闭过程。
	//   - 方法实现采用同步阻塞（blocking）方式，调用者会被挂起，直到所有 Actor 确认终止、资源完全释放并安全退出后才会返回。
	//   - 停止流程包括向所有活跃 Actor 派发终止信号（如 Poison Pill/FSM 终止），并确保子 Actor 优先于父 Actor 停止，递归释放所有托管的上下文与资源。
	//   - 用于应用生命周期管理，可保障关闭前所有未处理消息与状态持久化等任务优雅完成，防止资源泄漏及并发冲突。
	//
	// 注意事项：
	//   - 多次调用 Stop() 并无额外副作用，仅首个调用会触发实际终止流程，其余调用会在等待终止完成后直接返回。
	//   - 停止操作一经触发，不可逆转，系统不可再用于消息接收、Actor 创建等操作。
	Stop()
}

// PrimaryActorSystem 定义了“主”ActorSystem 的扩展接口，代表系统的具体实现，提供创建子 Actor 的能力。
//
// 主要职责与说明：
//   - 继承自 ActorSystem，具备 Actor 系统的所有核心功能（如消息派发、父子关系、消息通信等）。
//   - 提供 ActorOf 方法，使得主系统实例拥有直接动态创建新 Actor 的能力，通常仅用于顶层系统 Actor、根上下文及系统管理场景。
//   - 框架内部通常仅由 ActorSystem 的具体实现类型实现此接口，对外只暴露 ActorSystem，提升安全性、防止误用。
//   - 限制 ActorOf 由系统统一调度，保证每个 Actor 的子 Actor 只能通过其父上下文管理，确保运行时树状结构、并发安全与协程隔离。
//
// 用法：
//   - 通常通过 bootstrap.NewActorSystem 工厂函数获得 PrimaryActorSystem 实例，并创建首个顶层 Actor。
//   - 普通 ActorContext 通常只通过其自身 ActorContext.ActorOf 创建子 Actor，避免直接操作 PrimaryActorSystem 以破坏封装与安全性。
type PrimaryActorSystem interface {
	ActorSystem
	actorRace
}

// ActorSystemOption 定义了用于配置 ActorSystem 行为的函数类型。
// 调用方可通过一组 ActorSystemOption 配置项来定制系统初始化参数，实现灵活、可扩展的配置能力。
// 每个配置项均以函数方式实现，通过修改 ActorSystemOptions 结构体中的对应字段来生效。
type ActorSystemOption = func(options *ActorSystemOptions)

// ActorSystemOptions 封装了 ActorSystem 初始化和运行时的核心配置参数。
// 该结构体随着 ActorSystem 的创建流程被逐步填充，所有配置项均应通过 ActorSystemOption 配置函数进行设置。
// 增加新配置时，只需在此结构体内扩展字段，能够保证向后兼容与良好的扩展性。
type ActorSystemOptions struct {
	// DefaultAskTimeout 指定所有 Actor 在调用 Ask 模式（请求-应答）时的默认超时时长。
	// 若单次调用未特别指定，则将采用该超时时间，超时后会导致 Future 对象失败。
	// 合理配置此值可防止消息“悬挂”导致资源泄漏，也可根据业务特性灵活设置。
	DefaultAskTimeout time.Duration

	// Logger 指定 ActorSystem 的日志记录器。
	// 若未指定，则使用默认的日志记录器。
	Logger log.Logger
}

// WithActorSystemDefaultAskTimeout 返回一个 ActorSystemOption，用于指定 ActorSystem 的默认 Ask 超时时间。
//
// 用法场景：
//   - 在构建 ActorSystem 时，通过该 Option 明确设置全局默认的 Ask（请求-应答）操作超时阈值。
//   - 支持灵活的业务需求（如部分场景消息响应较慢时可延长超时，或测试环境下缩短等待时间）。
//
// 参数：
//   - timeout: 期望设置的超时时间，仅当 timeout > 0 时生效（不允许零值或负值；零/负值时忽略该配置）。
func WithActorSystemDefaultAskTimeout(timeout time.Duration) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		// 仅当指定的超时时长有效（大于零）时，才设置为默认 Ask 超时时间。
		// 无效值（零或负数）将被自动忽略，留用系统默认或上游已设值。
		if timeout > 0 {
			opts.DefaultAskTimeout = timeout
		}
	}
}

// WithActorSystemLogger 返回一个 ActorSystemOption，用于指定 ActorSystem 的日志记录器。
//
// 用法场景：
//   - 在构建 ActorSystem 时，通过该 Option 明确设置 ActorSystem 的日志记录器。
//   - 支持灵活的业务需求（如部分场景需要自定义日志记录器，或测试环境下使用内存日志记录器）。
//
// 参数：
//   - logger: 期望设置的日志记录器。
func WithActorSystemLogger(logger log.Logger) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		opts.Logger = logger
	}
}
