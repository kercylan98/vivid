package vivid

import (
	"slices"
	"time"

	"github.com/kercylan98/vivid/internal/sugar"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ FixedOptionActor = (*fixedOptionActor)(nil)
	_ PrelaunchActor   = (*prelaunchActor)(nil)
	_ PreRestartActor  = (*preRestartActor)(nil)
	_ RestartedActor   = (*restartedActor)(nil)
	_ Actor            = (*complexCombinationActor)(nil)
)

// uselessActor 占位空实现，组合 Actor 未提供实现时使用。
var (
	uselessActor Actor = ActorFN(func(ctx ActorContext) {})
)

// complexCombinationActor 将多个 Actor 顺序组合，依次委托 Prelaunch、PreRestart、Restarted、OnReceive。
type complexCombinationActor struct {
	actors []Actor
}

// NewComplexCombinationActor 将多个 Actor 组合为一个，按顺序调用各扩展接口与 OnReceive；nil 元素被忽略。
func NewComplexCombinationActor(actors ...Actor) Actor {
	return &complexCombinationActor{
		actors: slices.DeleteFunc(actors, func(actor Actor) bool {
			return actor == nil
		}),
	}
}

func (a *complexCombinationActor) FixedOptions(ctx FixedOptionContext) []ActorOption {
	for _, actor := range a.actors {
		if fixedOptionActor, ok := actor.(FixedOptionActor); ok {
			return fixedOptionActor.FixedOptions(ctx)
		}
	}
	return nil
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

// Actor 核心接口，实现方可被系统调度。
// OnReceive 在消息投递时被异步调用，典型实现根据 ctx.Message() 做类型分派。
type Actor interface {
	OnReceive(ctx ActorContext)
}

// ActorProvider 提供 Actor 实例的接口。
type ActorProvider interface {
	Provide() Actor
}

// ActorProviderFN 以函数实现 ActorProvider。
type ActorProviderFN func() Actor

func (fn ActorProviderFN) Provide() Actor {
	return fn()
}

// FixedOptionActor 扩展 Actor，在启动时注入固定选项（ActorOption 列表）。
type FixedOptionActor interface {
	Actor
	FixedOptions(ctx FixedOptionContext) []ActorOption
}

// fixedOptionActor 包装 Actor 并注入固定选项。
type fixedOptionActor struct {
	Actor
	fixedOptions []ActorOption
}

func (a *fixedOptionActor) FixedOptions(ctx FixedOptionContext) []ActorOption {
	return a.fixedOptions
}

// NewFixedOptionActor 返回带固定选项的 Actor；options 在启动时应用，未传 actor 时使用占位实现。
func NewFixedOptionActor(actor Actor, options ...ActorOption) Actor {
	if actor == nil {
		actor = uselessActor
	}
	return &fixedOptionActor{
		Actor:        actor,
		fixedOptions: options,
	}
}

// PrelaunchActor 扩展 Actor，在正式注册前执行一次 OnPrelaunch；返回非 nil error 则启动中止。
type PrelaunchActor interface {
	Actor
	OnPrelaunch(ctx PrelaunchContext) error
}

// prelaunchActor 包装 Actor 并注入预启动回调。
type prelaunchActor struct {
	Actor
	prelaunchHandler func(ctx PrelaunchContext) error
}

func (a *prelaunchActor) OnPrelaunch(ctx PrelaunchContext) error {
	return a.prelaunchHandler(ctx)
}

// NewPrelaunchActor 返回带预启动逻辑的 Actor；prelaunchHandler 在注册前执行一次，未传 actor 时使用占位实现。
func NewPrelaunchActor(prelaunchHandler func(ctx PrelaunchContext) error, actor ...Actor) Actor {
	if prelaunchHandler == nil {
		prelaunchHandler = func(ctx PrelaunchContext) error { return nil }
	}
	return &prelaunchActor{
		Actor:            sugar.FirstOrDefault(actor, uselessActor),
		prelaunchHandler: prelaunchHandler,
	}
}

// PreRestartActor 扩展 Actor，在重启前执行一次 OnPreRestart；返回非 nil error 则重启中止。
type PreRestartActor interface {
	Actor
	OnPreRestart(ctx RestartContext) error
}

// preRestartActor 包装 Actor 并注入预重启回调。
type preRestartActor struct {
	Actor
	preRestartHandler func(ctx RestartContext) error
}

func (a *preRestartActor) OnPreRestart(ctx RestartContext) error {
	return a.preRestartHandler(ctx)
}

// NewPreRestartActor 返回带预重启逻辑的 Actor；preRestartHandler 在重启前执行一次，未传 actor 时使用占位实现。
func NewPreRestartActor(preRestartHandler func(ctx RestartContext) error, actor ...Actor) Actor {
	if preRestartHandler == nil {
		preRestartHandler = func(ctx RestartContext) error { return nil }
	}
	return &preRestartActor{
		Actor:             sugar.FirstOrDefault(actor, uselessActor),
		preRestartHandler: preRestartHandler,
	}
}

// RestartedActor 扩展 Actor，在重启完成后执行一次 OnRestarted；返回 error 仅记录，不阻止运行。
type RestartedActor interface {
	Actor
	OnRestarted(ctx RestartContext) error
}

// restartedActor 包装 Actor 并注入重启后回调。
type restartedActor struct {
	Actor
	restartedHandler func(ctx RestartContext) error
}

func (a *restartedActor) OnRestarted(ctx RestartContext) error {
	return a.restartedHandler(ctx)
}

// NewRestartedActor 返回带重启后逻辑的 Actor；restartedHandler 在重启完成后执行一次，未传 actor 时使用占位实现。
func NewRestartedActor(restartedHandler func(ctx RestartContext) error, actor ...Actor) Actor {
	if restartedHandler == nil {
		restartedHandler = func(ctx RestartContext) error { return nil }
	}
	return &restartedActor{
		Actor:            sugar.FirstOrDefault(actor, uselessActor),
		restartedHandler: restartedHandler,
	}
}

// ActorFN 以函数实现 Actor。
type ActorFN func(ctx ActorContext)

func (fn ActorFN) OnReceive(ctx ActorContext) {
	fn(ctx)
}

// ActorOption 配置函数，用于修改 ActorOptions。推荐使用 WithXxx 构造器。
type ActorOption = func(options *ActorOptions)

// ActorOptions Actor 启动配置，通过 ActorOption 设置。
type ActorOptions struct {
	Name                string              // 名称，同一父下应唯一
	Mailbox             Mailbox             // 邮箱实现
	DefaultAskTimeout   time.Duration       // Ask 默认超时
	Logger              log.Logger          // 专用 Logger
	SupervisionStrategy SupervisionStrategy // 监督策略
	Provider            ActorProvider       // 重启时提供新实例；未设置则重启不替换实例
}

// WithActorSupervisionStrategy 设置监督策略，未设置时使用系统默认。
func WithActorSupervisionStrategy(supervisionStrategy SupervisionStrategy) ActorOption {
	return func(opts *ActorOptions) {
		opts.SupervisionStrategy = supervisionStrategy
	}
}

// WithActorOptions 用给定 ActorOptions 覆盖当前配置。
func WithActorOptions(options ActorOptions) ActorOption {
	return func(opts *ActorOptions) {
		*opts = options
	}
}

// WithActorName 设置名称，同一父下应唯一；未设置时由系统分配。
func WithActorName(name string) ActorOption {
	return func(opts *ActorOptions) {
		opts.Name = name
	}
}

// WithActorMailbox 设置邮箱，未设置时使用系统默认。
func WithActorMailbox(mailbox Mailbox) ActorOption {
	return func(opts *ActorOptions) {
		opts.Mailbox = mailbox
	}
}

// WithActorDefaultAskTimeout 设置 Ask 默认超时，仅 timeout>0 时生效，优先于系统默认。
func WithActorDefaultAskTimeout(timeout time.Duration) ActorOption {
	return func(opts *ActorOptions) {
		if timeout > 0 {
			opts.DefaultAskTimeout = timeout
		}
	}
}

// WithActorLogger 设置专用 Logger，未设置时使用系统默认。
func WithActorLogger(logger log.Logger) ActorOption {
	return func(opts *ActorOptions) {
		opts.Logger = logger
	}
}

// WithActorProvider 设置 Provider，重启时用于提供新实例；未设置则重启不替换实例。
func WithActorProvider(provider ActorProvider) ActorOption {
	return func(opts *ActorOptions) {
		opts.Provider = provider
	}
}
