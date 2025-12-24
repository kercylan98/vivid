package vivid

import (
	"time"

	"github.com/kercylan98/vivid/pkg/log"
)

// Actor 定义了所有 Actor 的核心接口。
//
// OnReceive 方法用于接收并处理传递给该 Actor 的消息。
// 每个自定义 Actor 类型必须实现该接口以保证能被 Actor 系统调度。
// 典型实现方式为匹配 ctx.Message() 类型处理不同业务消息。
type Actor interface {
	// OnReceive 由 ActorSystem 在有消息投递至该 Actor 时异步调用。
	// ctx 为当前请求的上下文，包含消息体及 Actor 环境。
	OnReceive(ctx ActorContext)
}

// PrelaunchActor 扩展了 Actor 接口，允许 Actor 在正式启动前执行预处理逻辑。
//
// OnPrelaunch 实现可用于依赖注入、资源预加载、配置校验等初始化前操作。
// 返回 error 表示启动失败，系统将阻止该 Actor 启动并记录错误。
type PrelaunchActor interface {
	Actor
	// OnPrelaunch 会在 Actor 正式注册前被调用，仅被调用一次。
	// 返回 error 则 Actor 启动流程将中断。
	OnPrelaunch() error
}

// ActorFN 是基于函数适配的 Actor 实现方式。
//
// 可通过直接传递函数实现 Actor 行为，简化简单业务场景下的类型定义。
// 例如：ActorFN(func(ctx ActorContext) { ... })。
type ActorFN func(ctx ActorContext)

// OnReceive 实现 Actor 接口，将消息调度委托给具体的函数实现。
func (fn ActorFN) OnReceive(ctx ActorContext) {
	fn(ctx)
}

// ActorOption 定义了 ActorOptions 的配置项函数类型。
//
// 可通过一组 ActorOption 对 ActorOptions 结构体进行灵活配置。
// 推荐使用 WithXxxx 风格的构造器对 Actor 启动参数进行设置。
type ActorOption = func(options *ActorOptions)

// ActorOptions 封装了 Actor 启动过程中的核心配置项。
//
// 包含 Actor 命名、邮箱实现、默认 Ask 超时，以及专属 Logger。
// 建议通过 ActorOption 构造器进行配置，便于向后兼容及灵活扩展。
type ActorOptions struct {
	Name              string        // Name 指定 Actor 的标识性名称（唯一性由父级上下文保证）。
	Mailbox           Mailbox       // Mailbox 指定 Actor 的消息邮箱实例，支持自定义调度模型。
	DefaultAskTimeout time.Duration // DefaultAskTimeout 指定该 Actor Ask 请求的默认超时时间。
	Logger            log.Logger    // Logger 为 Actor 专用日志对象，便于定位问题。
}

// WithActorOptions 返回一个设置完整 ActorOptions 的配置项。
// 通常用于批量重用 ActorOptions 结构体。
func WithActorOptions(options ActorOptions) ActorOption {
	return func(opts *ActorOptions) {
		*opts = options
	}
}

// WithActorName 返回一个设置 Actor.Name 字段的配置项。
//
// name 用于标识 Actor 实例，建议在同一父级下具备唯一性。
// 若未指定，则由系统自动分配。
func WithActorName(name string) ActorOption {
	return func(opts *ActorOptions) {
		opts.Name = name
	}
}

// WithActorMailbox 返回一个设置 Actor.Mailbox 字段的配置项。
//
// mailbox 允许注入自定义邮箱模型，实现消息优先级、自定义调度等需求。
// 若未指定，则采用系统默认邮箱实现。
func WithActorMailbox(mailbox Mailbox) ActorOption {
	return func(opts *ActorOptions) {
		opts.Mailbox = mailbox
	}
}

// WithActorDefaultAskTimeout 返回一个设置 Actor.DefaultAskTimeout 字段的配置项。
//
// timeout 仅在大于零时生效，优先级高于系统级超时时间。
// 常用场景为依赖外部接口或需定制响应 SLA 的 Actor。
func WithActorDefaultAskTimeout(timeout time.Duration) ActorOption {
	return func(opts *ActorOptions) {
		if timeout > 0 {
			opts.DefaultAskTimeout = timeout
		}
	}
}

// WithActorLogger 返回一个设置 Actor.Logger 字段的配置项。
//
// logger 可为每个 Actor 提供独立日志输出，以便隔离追踪及模块化管理。
// 未指定时，通常由系统注入默认 Logger。
func WithActorLogger(logger log.Logger) ActorOption {
	return func(opts *ActorOptions) {
		opts.Logger = logger
	}
}
