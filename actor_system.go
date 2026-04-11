package vivid

import (
	"context"
	"time"

	"github.com/kercylan98/vivid/internal/sugar"
	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/metrics"
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
//   - 推荐通过 NewActorSystem（见 bootstrap 包）工厂方法创建实例，并使用 error 进行错误处理与解包。
//
// 注意事项：
//   - ActorSystem 实例设计为轻量且线程安全，避免作为全局变量暴露在多线程环境下共享。
//   - 所有 Actor 的创建、消息调度、行为切换等应严格由所属 ActorSystem 实现和调度，保证隔离与安全。
type ActorSystem interface {
	actorBasic // 内嵌 actorCore 接口，继承 Actor 系统基础能力

	// Start 启动当前 ActorSystem 实例。
	//
	// 主要功能与行为说明：
	//   - 调用后会启动 ActorSystem 及其全部托管的 Actor（包括根 Actor 及所有子 Actor）。
	//   - 方法实现采用同步阻塞（blocking）方式，调用者会被挂起，直到所有 Actor 启动完成并返回。
	//   - 启动流程包括初始化 ActorSystem 及其内部组件，如远程通信、指标收集等。
	//   - 用于应用生命周期管理，可保障启动前所有未处理消息与状态持久化等任务优雅完成，防止资源泄漏及并发冲突。
	//   - 支持可选的超时参数，用于控制启动过程的时间限制。若超时，系统会立即终止并返回错误。
	Start() error

	// Stop 优雅地停止当前 ActorSystem 实例。
	//
	// 主要功能与行为说明：
	//   - 调用后会触发 ActorSystem 及其全部托管的 Actor（包括根 Actor 及所有子 Actor）的有序关闭过程。
	//   - 方法实现采用同步阻塞（blocking）方式，调用者会被挂起，直到所有 Actor 确认终止、资源完全释放并安全退出后才会返回。
	//   - 停止流程包括向所有活跃 Actor 派发终止信号（如 Poison Pill/FSM 终止），并确保子 Actor 优先于父 Actor 停止，递归释放所有托管的上下文与资源。
	//   - 用于应用生命周期管理，可保障关闭前所有未处理消息与状态持久化等任务优雅完成，防止资源泄漏及并发冲突。
	//   - 支持可选的超时参数，用于控制停止过程的时间限制。若超时，系统会立即终止并返回错误。
	//
	// 注意事项：
	//   - 多次调用 Stop() 并无额外副作用，仅首个调用会触发实际终止流程，其余调用会在等待终止完成后直接返回。
	//   - 停止操作一经触发，不可逆转，系统不可再用于消息接收、Actor 创建等操作。
	Stop(timeout ...time.Duration) error

	// FindActor 根据引用字符串查找本节点上已存在的 Actor 并返回其引用。
	// 仅支持本机地址：若字符串指向远程节点，或本机不存在该路径的 Actor，则返回错误。
	// 用于“确认本机有该 Actor 并拿到其引用”的场景。
	FindActor(actorRef string) (ActorRef, error)

	// ParseRef 将引用字符串解析为 ActorRef，不要求目标存在于本节点或远程。
	// 仅做格式解析，用于配置、服务发现中拿到的字符串需要发消息时（本地或远程均可）。
	ParseRef(actorRef string) (ActorRef, error)

	// CreateRef 根据地址与路径构造 ActorRef，不要求目标存在；用于本地或远程引用（如集群单例、远程节点）。
	CreateRef(address string, path string) (ActorRef, error)

	// ActorOf 该方法的效果与 ActorContext.ActorOf 相同，但是它是并发安全的。
	ActorOf(actor Actor, options ...ActorOption) (ActorRef, error)

	// VirtualRef 创建一个虚拟 Actor 的引用。
	VirtualRef(kind string, name string) ActorRef

	// Probe 探测指定 Actor 心跳状态
	Probe(ref ActorRef, timeout ...time.Duration) Future[*Heartbeat]
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

func NewActorSystemOptions(options ...ActorSystemOption) *ActorSystemOptions {
	options = append([]ActorSystemOption{
		WithActorSystemContext(context.Background()),
		WithActorSystemDefaultAskTimeout(DefaultAskTimeout),
		WithActorSystemLogger(log.GetDefault()),
		WithActorSystemEnableMetricsUpdatedNotify(-1),
		WithActorSystemStopTimeout(time.Minute),
		WithActorSystemSupervisionStrategy(defaultSupervisionStrategy),
		WithActorSystemRemotingOptions(NewActorSystemRemotingOptions()),
	}, options...)

	opts := &ActorSystemOptions{}

	// 适配默认 AdvertiseAddress 为 BindAddress 的场景
	if opts.RemotingBindAddress != "" && opts.RemotingAdvertiseAddress == "" {
		opts.RemotingAdvertiseAddress = opts.RemotingBindAddress
	}

	for _, option := range options {
		option(opts)
	}

	return opts
}

// ActorSystemOptions 封装了 ActorSystem 初始化和运行时的核心配置参数。
// 该结构体随着 ActorSystem 的创建流程被逐步填充，所有配置项均应通过 ActorSystemOption 配置函数进行设置。
// 增加新配置时，只需在此结构体内扩展字段，能够保证向后兼容与良好的扩展性。
type ActorSystemOptions struct {
	RemotingOptions *ActorSystemRemotingOptions

	// Context 指定 ActorSystem 的上下文。
	// 若未指定，则使用默认的上下文。
	Context context.Context

	// Logger 指定 ActorSystem 的日志记录器。
	// 若未指定，则使用默认的日志记录器。
	Logger log.Logger

	// RemotingCodec 指定用于远程通讯的消息编解码器。
	RemotingCodec Codec

	// Metrics 指标收集器。
	Metrics metrics.Metrics

	// RemotingBindAddress 指定远程通信的绑定地址。
	// 框架将在此地址上启动Listener接收连接。
	RemotingBindAddress string

	// RemotingAdvertiseAddress 指定远程通信的广告地址。
	// 用于标识本系统的网络地址，供其他系统连接。
	// TCP和UDP将复用同一端口。
	RemotingAdvertiseAddress string

	// DefaultAskTimeout 指定所有 Actor 在调用 Ask 模式（请求-应答）时的默认超时时长。
	// 若单次调用未特别指定，则将采用该超时时间，超时后会导致 Future 对象失败。
	// 合理配置此值可防止消息"悬挂"导致资源泄漏，也可根据业务特性灵活设置。
	DefaultAskTimeout time.Duration

	// EnableMetricsUpdatedNotify 指定是否启用指标收集更新通知。
	EnableMetricsUpdatedNotify time.Duration

	// StopTimeout 指定 ActorSystem 停止操作的超时时间。
	StopTimeout time.Duration

	// EnableMetrics 指定是否启用指标收集。
	// 启用后，系统会自动创建 Metrics Actor 来收集和统计系统运行指标。
	EnableMetrics bool

	// SupervisionStrategy 指定 ActorSystem 默认的监督策略。
	SupervisionStrategy SupervisionStrategy

	// VirtualActorProviders 指定虚拟 Actor 的提供者。
	//
	// 其中 key 为虚拟 Actor 的种类，value 为 VirtualActorProvider。
	VirtualActorProviders map[string]ActorProvider

	// MessageRegister 指定消息注册器。
	MessageRegister []MessageRegister

	// ClusterOptions 指定 ActorSystem 的集群选项。
	ClusterOptions *ActorSystemClusterOptions
}

// SystemBasicState 当前 ActorSystem 的基本状态，供控制台等展示；不含集群名称（仅集群有集群名）。
type SystemBasicState struct {
	StartTime       time.Time `json:"startTime"`       // 启动时间
	Version         string    `json:"version"`         // Vivid 库版本号，来自包常量 Version
	RemotingEnabled bool      `json:"remotingEnabled"` // 是否开启远程
	RemotingAddress string    `json:"remotingAddress"` // 远程广告地址，未开启时为空
	MetricsEnabled  bool      `json:"metricsEnabled"`  // 是否启用指标收集（WithActorSystemEnableMetrics）
}

// SystemStateProvider 可由 ActorSystem 实现，用于提供系统基本状态（启动时间、版本、是否开启远程等）。
type SystemStateProvider interface {
	GetSystemBasicState() SystemBasicState
}

// MetricsProvider 可由 ActorSystem 实现，用于提供指标收集器；配合 EnableMetricsUpdatedNotify 实现指标展示。
type MetricsProvider interface {
	IsMetricsEnabled() bool
	Metrics() metrics.Metrics
}

// WithActorSystemSupervisionStrategy 返回一个 ActorSystemOption，用于指定 ActorSystem 的监督策略。
//
// 用法场景：
//   - 在构建 ActorSystem 时，通过该 Option 明确设置 ActorSystem 的监督策略。
//   - 支持灵活的业务需求（如部分场景需要设置 ActorSystem 的监督策略，或测试环境下设置 ActorSystem 的监督策略）。
//
// 参数：
//   - supervisionStrategy: 期望设置的监督策略。
//     如果传入 nil，则会自动使用系统的默认监督策略（defaultSupervisionStrategy）。
func WithActorSystemSupervisionStrategy(supervisionStrategy SupervisionStrategy) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		if supervisionStrategy == nil {
			supervisionStrategy = defaultSupervisionStrategy
		}
		opts.SupervisionStrategy = supervisionStrategy
	}
}

// WithActorSystemOptions 返回一个 ActorSystemOption，用于设置 ActorSystem 的选项。
//
// 用法场景：
//   - 在构建 ActorSystem 时，通过该 Option 明确设置 ActorSystem 的选项。
//   - 支持灵活的业务需求（如部分场景需要设置 ActorSystem 的选项，或测试环境下设置 ActorSystem 的选项）。
//
// 参数：
//   - options: 期望设置的选项。
func WithActorSystemOptions(options *ActorSystemOptions) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		*opts = *options
	}
}

// WithActorSystemStopTimeout 返回一个 ActorSystemOption，用于指定 ActorSystem 停止操作的超时时间。
//
// 用法场景：
//   - 在构建 ActorSystem 时，通过该 Option 明确设置 ActorSystem 停止操作的超时时间。
//   - 支持灵活的业务需求（如部分场景需要延长停止时间，或测试环境下缩短停止时间）。
//
// 参数：
//   - timeout: 期望设置的超时时间，仅当 timeout > 0 时生效（不允许零值或负值；零/负值时忽略该配置）。
//
// 其他：假设通过直接构建 ActorSystemOptions 的情况设置了 <= 0 的值，将为导致停止后立刻触发超时。
func WithActorSystemStopTimeout(timeout time.Duration) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		if timeout > 0 {
			opts.StopTimeout = timeout
		}
	}
}

// WithActorSystemContext 返回一个 ActorSystemOption，用于指定 ActorSystem 的上下文。
//
// 用法场景：
//   - 在构建 ActorSystem 时，通过该 Option 明确设置 ActorSystem 的上下文。
//
// 参数：
//   - context: 期望设置的上下文。
func WithActorSystemContext(context context.Context) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		opts.Context = context
	}
}

// WithActorSystemEnableMetricsUpdatedNotify 返回一个 ActorSystemOption，用于设置指标更新时快照推送行为的间隔策略。
//
// 配置说明：
//   - 当 duration == 0 时：每次指标发生更新后，都会立即将最新指标快照（metrics.MetricsSnapshot）推送到事件流（EventStream）。
//   - 当 duration > 0 时：系统将按照指定的间隔定期推送指标快照到事件流，而不是每次变更都推送。
//   - 当 duration < 0 时：关闭指标快照推送功能（默认不开启）。
//
// 常用场景：
//   - 实时采集与推送：设置为 0，可用于需要及时响应指标变化的场合，例如开发调试或高敏感监控。
//   - 定时采集推送：设置为正值（如 5 秒），便于生产环境定期快照，减少推送频率和资源占用。
//   - 完全关闭：设置为负值，在无需指标变更通知时关闭（默认不开启）。
//
// 参数：
//   - duration: 指标推送间隔，具体行为见上方说明。
func WithActorSystemEnableMetricsUpdatedNotify(duration time.Duration) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		opts.EnableMetricsUpdatedNotify = duration
	}
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

// WithActorSystemEnableMetrics 返回一个 ActorSystemOption，用于启用指标收集功能。
//
// 启用后，系统会自动创建 Metrics Actor 来收集和统计系统运行指标，
// 包括 Actor 数量、失败数、重启数、死信数等核心指标。
//
// 用法场景：
//   - 在生产环境中启用指标收集，用于监控和诊断
//   - 通过 Metrics 接口查询系统运行状态
//
// 参数：
//   - enable: 是否启用指标收集，默认为 false
func WithActorSystemEnableMetrics(enable bool) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		opts.EnableMetrics = enable
		if opts.Metrics == nil {
			opts.Metrics = metrics.NewDefaultMetrics()
		}
	}
}

// WithActorSystemMetrics 返回一个 ActorSystemOption，用于指定指标收集器。
//
// 用法场景：
//   - 在构建 ActorSystem 时，通过该 Option 明确设置指标收集器。
//   - 支持灵活的业务需求（如部分场景需要自定义指标收集器，或测试环境下使用内存指标收集器）。
//
// 参数：
//   - metrics: 期望设置的指标收集器。
func WithActorSystemMetrics(metrics metrics.Metrics) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		opts.Metrics = metrics
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

// WithActorSystemRemoting 提供 ActorSystemOption，用于配置远程通信组件的监听及广告地址。
//
// 注意：如果需要跨网络进行消息序列化，必须通过 WithCodec 显式指定 Codec，
//
//	或者为所有自定义消息通过 RegisterCustomMessage 注册对应的消息读写器，
//	否则消息无法被正确地序列化和反序列化，导致分布式或远程通信失败。
//
// 用途说明：
//  1. 指定系统用于侦听远程连接的网络绑定地址（bindAddr），系统内部会自动初始化并管理 Listener 生命周期。
//  2. 可选设置对外公布（广告）的网络地址（advertiseAddr），常用于集群、NAT、端口映射等场景；若未指定，默认使用 bindAddr。
//
// 参数：
//   - bindAddr: string，必选，远程 Listener 的本地绑定地址（如 TCP/UDP 地址）。
//   - advertiseAddr: ...string，可选，对外广告地址（第一个参数有效），否则默认使用 bindAddr。
func WithActorSystemRemoting(bindAddr string, advertiseAddr ...string) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		opts.RemotingBindAddress = bindAddr
		opts.RemotingAdvertiseAddress = sugar.FirstOrDefault(advertiseAddr, bindAddr)
		if utils.IsAddrMissingPort(opts.RemotingAdvertiseAddress) && !utils.IsDomainName(opts.RemotingAdvertiseAddress) {
			panic("ActorSystem advertise address must be a domain when missing port")
		}
	}
}

// WithActorSystemClusterOption 返回一个 ActorSystemOption，用于配置 ActorSystem 的集群选项。
//
// 参数：
//   - options: 期望设置的集群选项。
func WithActorSystemgClusterOption(options ...ActorSystemClusterOption) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		if opts.ClusterOptions == nil {
			return
		}
		opts.ClusterOptions = NewActorSystemClusterOptions(options...)
	}
}

// WithActorSystemCodec 提供 ActorSystemOption，用于配置远程消息的序列化与反序列化 Codec。
//
// 如果希望 ActorSystem 支持跨网络或分布式消息传递，必须通过本选项显式设置 Codec，
// 否则需要对所有自定义消息类型调用 RegisterCustomMessage 注册对应的消息读写器。
// 否则系统无法完成消息的跨节点编解码，导致远程通信失败。
//
// 参数：
//   - codec: Codec 实例，必需用于远程消息序列化；若为 nil 会 panic。
func WithActorSystemCodec(codec Codec) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		if codec == nil {
			panic("ActorSystem Codec (WithCodec) must not be nil: required for cross-network message serialization or register custom message readers/writers with RegisterCustomMessage.")
		}
		opts.RemotingCodec = codec
	}
}

// WithActorSystemVirtualActorProvider 返回一个 ActorSystemOption，用于配置虚拟 Actor 的提供者。
//
// 参数：
//   - kind: 虚拟 Actor 的种类。
//   - provider: 虚拟 Actor 的提供者。
//
// 如果 kind 已存在，将会发生 panic。
func WithActorSystemVirtualActorProvider(kind string, provider ActorProvider) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		if provider == nil {
			return
		}
		if opts.VirtualActorProviders == nil {
			opts.VirtualActorProviders = make(map[string]ActorProvider)
		}
		if _, ok := opts.VirtualActorProviders[kind]; ok {
			panic("virtual actor provider already exists for kind: " + kind)
		}
		opts.VirtualActorProviders[kind] = provider
	}
}

func WithActorSystemMessageRegister(register MessageRegister) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		if register == nil {
			return
		}
		opts.MessageRegister = append(opts.MessageRegister, register)
	}
}
