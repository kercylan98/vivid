package vivid

import (
	"slices"
	"time"

	"github.com/kercylan98/vivid/internal/sugar"
	"github.com/kercylan98/vivid/pkg/log"
)

// 编译期接口实现校验。
var (
	_ PrelaunchActor  = (*prelaunchActor)(nil)
	_ PreRestartActor = (*preRestartActor)(nil)
	_ RestartedActor  = (*restartedActor)(nil)
	_ Actor           = (*complexCombinationActor)(nil)
)

// uselessActor 为占位用空实现，在组合 Actor 未提供实际实现时使用。
var (
	uselessActor Actor = ActorFN(func(ctx ActorContext) {})
)

// complexCombinationActor 将多个 Actor 组合为一个，按顺序委托 Prelaunch、PreRestart、Restarted 与 OnReceive。
type complexCombinationActor struct {
	actors []Actor
}

// NewComplexCombinationActor 将多个 Actor 组合为单个 Actor，各扩展接口与 OnReceive 会按顺序调用。
// 如果传入的 Actor 为 nil，则会被忽略。
func NewComplexCombinationActor(actors ...Actor) Actor {
	return &complexCombinationActor{
		actors: slices.DeleteFunc(actors, func(actor Actor) bool {
			return actor == nil
		}),
	}
}

func (a *complexCombinationActor) OnPrelaunch(ctx PrelaunchContext) error {
	for _, actor := range a.actors {
		if prelaunchActor, ok := actor.(PrelaunchActor); ok {
			if err := prelaunchActor.OnPrelaunch(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *complexCombinationActor) OnPreRestart(ctx RestartContext) error {
	for _, actor := range a.actors {
		if preRestartActor, ok := actor.(PreRestartActor); ok {
			if err := preRestartActor.OnPreRestart(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *complexCombinationActor) OnRestarted(ctx RestartContext) error {
	for _, actor := range a.actors {
		if restartedActor, ok := actor.(RestartedActor); ok {
			if err := restartedActor.OnRestarted(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *complexCombinationActor) OnReceive(ctx ActorContext) {
	for _, actor := range a.actors {
		actor.OnReceive(ctx)
	}
}

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

// ActorProvider 定义了 Actor 的提供者接口。
type ActorProvider interface {
	// Provide 提供 Actor 实例。
	Provide() Actor
}

// ActorProviderFN 是基于函数适配的 ActorProvider 实现方式。
// 可通过直接传递函数实现 ActorProvider 行为，简化简单业务场景下的类型定义。
// 例如：ActorProviderFN(func() Actor { ... })。
type ActorProviderFN func() Actor

// Provide 实现 ActorProvider 接口，将 Actor 实例的提供委托给具体的函数实现。
func (fn ActorProviderFN) Provide() Actor {
	return fn()
}

// PrelaunchActor 扩展了 Actor 接口，允许 Actor 在正式启动前执行预处理逻辑。
//
// OnPrelaunch 实现可用于依赖注入、资源预加载、配置校验等初始化前操作。
// 返回 error 表示启动失败，系统将阻止该 Actor 启动并记录错误。
type PrelaunchActor interface {
	Actor
	// OnPrelaunch 会在 Actor 正式注册前被调用，仅被调用一次。
	// 返回 error 则 Actor 启动流程将中断。
	OnPrelaunch(ctx PrelaunchContext) error
}

// prelaunchActor 包装一个 Actor 并注入预启动回调，用于实现 PrelaunchActor。
type prelaunchActor struct {
	Actor
	prelaunchHandler func(ctx PrelaunchContext) error
}

func (a *prelaunchActor) OnPrelaunch(ctx PrelaunchContext) error {
	return a.prelaunchHandler(ctx)
}

// NewPrelaunchActor 用于快速创建一个带有预启动逻辑（Prelaunch）的 Actor 实例。
// prelaunchHandler 在 Actor 正式启动前执行，常用于依赖注入、配置校验、资源预加载等。
// actor 为实际的业务 Actor 实现；未传入时使用占位空实现。
// 返回实现了 PrelaunchActor 的 Actor，系统会在启动时优先执行 prelaunchHandler，若返回错误则中断启动。
func NewPrelaunchActor(prelaunchHandler func(ctx PrelaunchContext) error, actor ...Actor) Actor {
	if prelaunchHandler == nil {
		prelaunchHandler = func(ctx PrelaunchContext) error { return nil }
	}
	return &prelaunchActor{
		Actor:            sugar.FirstOrDefault(actor, uselessActor),
		prelaunchHandler: prelaunchHandler,
	}
}

// PreRestartActor 扩展了 Actor 接口，允许 Actor 在重启前执行预处理逻辑。
//
// OnPreRestart 实现可用于重启前的资源清理、状态保存、条件校验等。
// 返回 error 表示重启失败，系统将阻止该 Actor 重启并记录错误。
type PreRestartActor interface {
	Actor
	// OnPreRestart 会在 Actor 重启前被调用，仅被调用一次。
	OnPreRestart(ctx RestartContext) error
}

// preRestartActor 包装一个 Actor 并注入预重启回调，用于实现 PreRestartActor。
type preRestartActor struct {
	Actor
	preRestartHandler func(ctx RestartContext) error
}

func (a *preRestartActor) OnPreRestart(ctx RestartContext) error {
	return a.preRestartHandler(ctx)
}

// NewPreRestartActor 用于快速创建一个带有预重启逻辑（PreRestart）的 Actor 实例。
// preRestartHandler 在 Actor 重启前执行，常用于资源清理、状态保存、条件校验等。
// actor 为实际的业务 Actor 实现；未传入时使用占位空实现。
// 返回实现了 PreRestartActor 的 Actor，系统会在重启前执行 preRestartHandler，若返回错误则中断重启。
func NewPreRestartActor(preRestartHandler func(ctx RestartContext) error, actor ...Actor) Actor {
	if preRestartHandler == nil {
		preRestartHandler = func(ctx RestartContext) error { return nil }
	}
	return &preRestartActor{
		Actor:             sugar.FirstOrDefault(actor, uselessActor),
		preRestartHandler: preRestartHandler,
	}
}

// RestartedActor 扩展了 Actor 接口，允许 Actor 在重启完成后执行恢复逻辑。
//
// OnRestarted 实现可用于状态恢复、资源重新初始化等重启后操作。
// 返回 error 会被记录，但不会阻止 Actor 继续运行。
type RestartedActor interface {
	Actor
	// OnRestarted 会在 Actor 重启完成后被调用，仅被调用一次。
	OnRestarted(ctx RestartContext) error
}

// restartedActor 包装一个 Actor 并注入重启后回调，用于实现 RestartedActor。
type restartedActor struct {
	Actor
	restartedHandler func(ctx RestartContext) error
}

func (a *restartedActor) OnRestarted(ctx RestartContext) error {
	return a.restartedHandler(ctx)
}

// NewRestartedActor 用于快速创建一个带有重启后逻辑（Restarted）的 Actor 实例。
// restartedHandler 在 Actor 重启完成后执行，常用于状态恢复、资源重新初始化等。
// actor 为实际的业务 Actor 实现；未传入时使用占位空实现。
// 返回实现了 RestartedActor 的 Actor，系统会在重启完成后调用 restartedHandler。
func NewRestartedActor(restartedHandler func(ctx RestartContext) error, actor ...Actor) Actor {
	if restartedHandler == nil {
		restartedHandler = func(ctx RestartContext) error { return nil }
	}
	return &restartedActor{
		Actor:            sugar.FirstOrDefault(actor, uselessActor),
		restartedHandler: restartedHandler,
	}
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
	Name                string              // 指定 Actor 的标识性名称（唯一性由父级上下文保证）。
	Mailbox             Mailbox             // 指定 Actor 的消息邮箱实例，支持自定义调度模型。
	DefaultAskTimeout   time.Duration       // 指定该 Actor Ask 请求的默认超时时间。
	Logger              log.Logger          // 为 Actor 专用日志对象，便于定位问题。
	SupervisionStrategy SupervisionStrategy // 指定 Actor 的监督策略。
	Provider            ActorProvider       // 指定 Actor 的提供者，在 Actor 重启时用于提供新实例；未指定则不会在重启时替换实例，仍可在其生命周期内主动重置。
}

// WithActorSupervisionStrategy 返回一个设置 Actor.SupervisionStrategy 字段的配置项。
//
// supervisionStrategy 为 Actor 的监督策略。如果未指定，则使用系统默认的监督策略。
//
// 返回:
//   - ActorOption: 一个设置 Actor.SupervisionStrategy 字段的配置项。
func WithActorSupervisionStrategy(supervisionStrategy SupervisionStrategy) ActorOption {
	return func(opts *ActorOptions) {
		opts.SupervisionStrategy = supervisionStrategy
	}
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

// WithActorProvider 返回一个设置 Actor.Provider 字段的配置项。
//
// provider 在 Actor 重启时用于提供新实例；未指定则不会在重启时替换实例，仍可在其生命周期内主动重置。
func WithActorProvider(provider ActorProvider) ActorOption {
	return func(opts *ActorOptions) {
		opts.Provider = provider
	}
}
