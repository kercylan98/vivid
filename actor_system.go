package vivid

import (
	"context"
	"time"

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

	// FindActorRef 根据字符串解析生成 ActorRef 实例。
	// 参数：
	//   - actorRef: actor 引用字符串（如 "example.com:8080/user/a"）。
	// 返回值：
	//   - vivid.ActorRef: 解析得到的 actor 引用对象，若解析失败则为 nil。
	//   - error: 字符串格式、地址或路径非法时返回对应错误。
	//
	// 用于把存储、传输的字符串形式 actor ref 转为可用的 ActorRef 对象。
	FindActorRef(actorRef string) (ActorRef, error)

	// ActorOf 该方法的效果与 ActorContext.ActorOf 相同，但是它是并发安全的。
	ActorOf(actor Actor, options ...ActorOption) (ActorRef, error)
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
	RemotingOptions ActorSystemRemotingOptions

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
}

// WithActorSystemStopTimeout 返回一个 ActorSystemOption，用于指定 ActorSystem 停止操作的超时时间。
//
// 用法场景：
//   - 在构建 ActorSystem 时，通过该 Option 明确设置 ActorSystem 停止操作的超时时间。
//   - 支持灵活的业务需求（如部分场景需要延长停止时间，或测试环境下缩短停止时间）。
//
// 参数：
//   - timeout: 期望设置的超时时间，仅当 timeout > 0 时生效（不允许零值或负值；零/负值时忽略该配置）。
func WithActorSystemStopTimeout(timeout time.Duration) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		opts.StopTimeout = timeout
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
		if len(advertiseAddr) > 0 {
			opts.RemotingAdvertiseAddress = advertiseAddr[0]
		} else {
			opts.RemotingAdvertiseAddress = bindAddr
		}
		if utils.IsAddrMissingPort(opts.RemotingAdvertiseAddress) && !utils.IsDomainName(opts.RemotingAdvertiseAddress) {
			panic("ActorSystem advertise address must be a domain when missing port")
		}
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

// ActorSystemRemotingConnectionReadFailedHandler 定义远程连接读取失败的处理器接口。
// 实现该接口的类型可用于自定义当系统检测到远程连接读取操作发生错误时的处理逻辑，例如网络异常、对端断开连接等，
// 以实现自定义的重试、报警或容错策略。
//
// 方法说明：
//   - HandleRemotingConnectionReadFailed: 当远程连接读取失败时被调用；
//   - fatal: 表示此次错误是否为致命错误，若为 true 通常需要采取断开连接、关闭 ActorSystem 或自定义降级策略；
//   - err:   捕获到的具体错误对象；
//
// 返回值：对于非致命错误（fatal=false），若返回非 nil error，系统会将当前远程连接停止，并将该 error 作为停止原因(StopReason)。
//   - 若 fatal=true，error 作为致命错误处理（如关闭整个 ActorSystem）；
//   - 可返回 nil 跳过后续处理。
type ActorSystemRemotingConnectionReadFailedHandler interface {
	HandleRemotingConnectionReadFailed(fatal bool, err error) error
}

// ActorSystemRemotingConnectionReadFailedHandlerFN 是适配函数式处理器的类型。
// 允许使用函数直接实现 ActorSystemRemotingConnectionReadFailedHandler 接口，提升易用性和灵活性。
type ActorSystemRemotingConnectionReadFailedHandlerFN func(fatal bool, err error) error

// HandleRemotingConnectionReadFailed 调用底层函数本体，实现接口契约。
func (h ActorSystemRemotingConnectionReadFailedHandlerFN) HandleRemotingConnectionReadFailed(fatal bool, err error) error {
	return h(fatal, err)
}

// ActorSystemRemotingOption 定义一个用来配置 ActorSystemRemotingOptions 的函数签名。
// 开发者可通过一组链式 Option 函数灵活配置远程通信相关的高级参数，实现高度可扩展的定制能力。
type ActorSystemRemotingOption func(options *ActorSystemRemotingOptions)

// ActorSystemRemotingOptions 封装了 ActorSystem 远程通信组件在运行时的选项参数。
// 新增远程相关的可扩展参数时，建议集中在本结构体内按需扩展，以实现更好的向前兼容和配置集中管理。
type ActorSystemRemotingOptions struct {
	// ConnectionReadFailedHandler 用于处理系统级的远程连接读取失败事件。
	// 可设置为自定义实现，或使用 ActorSystemRemotingConnectionReadFailedHandlerFN。
	ConnectionReadFailedHandler ActorSystemRemotingConnectionReadFailedHandler
}

// WithActorSystemRemotingOptions 返回一个 ActorSystemOption，用于批量配置 ActorSystem 远程通信的高级选项。
//
// 用法说明：
//   - 首个参数为一个 ActorSystemRemotingOptions 结构体，用于初始化远程选项的默认值；
//   - 其余可变参数为 ActorSystemRemotingOption 函数，可链式定制具体配置；
//   - 推荐通过该方法集中配置包括远程异常、错误处理、重连、连接池等扩展能力。
//
// 典型用法：
//
//	WithActorSystemRemotingOptions(
//	    ActorSystemRemotingOptions{
//	        ConnectionReadFailedHandler: myHandler,
//	    },
//	    func(opt *ActorSystemRemotingOptions) { /* 其它自定义扩展 */ },
//	)
//
// 参数：
//   - options:            远程通信选项的初始配置。
//   - opts ...ActorSystemRemotingOption: 可选参数，链式扩展远程通信选项。
//
// 返回值：
//   - ActorSystemOption:  可传给 NewActorSystem 或其它配置参数的 Option 函数。
func WithActorSystemRemotingOptions(options ActorSystemRemotingOptions, opts ...ActorSystemRemotingOption) ActorSystemOption {
	return func(o *ActorSystemOptions) {
		o.RemotingOptions = options
		for _, opt := range opts {
			opt(&o.RemotingOptions)
		}
	}
}
