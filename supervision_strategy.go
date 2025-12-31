package vivid

import (
	"fmt"
	"time"

	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/pkg/log"
)

// SupervisionDecision 定义了监督决策的类型，用于在监督过程中选择不同的处理方式。
//
// 可通过常量 SupervisionDecisionRestart、SupervisionDecisionStop、SupervisionDecisionResume 和 SupervisionDecisionEscalate 访问具体的决策值。
//
// 可通过 String 方法将监督决策转换为字符串表示。
//
// 在预料之外的决策值将作为升级处理。
type SupervisionDecision int8

const (
	SupervisionDecisionRestart         SupervisionDecision = iota + 1 // 重启
	SupervisionDecisionGracefulRestart                                // 优雅重启
	SupervisionDecisionStop                                           // 停止
	SupervisionDecisionGracefulStop                                   // 优雅停止
	SupervisionDecisionResume                                         // 恢复
	SupervisionDecisionEscalate                                       // 升级
)

// IsGraceful 判断当前监督决策是否为优雅执行。
func (decision SupervisionDecision) IsGraceful() bool {
	return decision == SupervisionDecisionGracefulRestart || decision == SupervisionDecisionGracefulStop
}

// IsRestart 判断当前监督决策是否为重启。
func (decision SupervisionDecision) IsRestart() bool {
	return decision == SupervisionDecisionRestart || decision == SupervisionDecisionGracefulRestart
}

// IsStop 判断当前监督决策是否为停止。
func (decision SupervisionDecision) IsStop() bool {
	return decision == SupervisionDecisionStop || decision == SupervisionDecisionGracefulStop
}

// IsResume 判断当前监督决策是否为恢复。
func (decision SupervisionDecision) IsResume() bool {
	return decision == SupervisionDecisionResume
}

// IsEscalate 判断当前监督决策是否为升级。
func (decision SupervisionDecision) IsEscalate() bool {
	return decision == SupervisionDecisionEscalate
}

// IsValid 判断当前监督决策是否为有效。
func (decision SupervisionDecision) IsValid() bool {
	return decision > 0 && decision <= SupervisionDecisionEscalate
}

// String 返回当前监督决策的字符串表示。
// 如果决策为未知，则返回 "escalate(unknown: <决策值>)"。
func (decision SupervisionDecision) String() string {
	switch decision {
	case SupervisionDecisionRestart:
		return "restart"
	case SupervisionDecisionGracefulRestart:
		return "restart(graceful)"
	case SupervisionDecisionStop:
		return "stop"
	case SupervisionDecisionGracefulStop:
		return "stop(graceful)"
	case SupervisionDecisionResume:
		return "resume"
	case SupervisionDecisionEscalate:
		return "escalate"
	default:
		return fmt.Sprintf("escalate(unknown: %d)", decision)
	}
}

// SupervisionStrategyDecisionMaker 定义了监督策略决策器接口，用于根据当前监督上下文做出决策。
type SupervisionStrategyDecisionMaker interface {
	// MakeDecision 方法用于根据当前监督上下文做出决策，返回监督决策和原因。
	MakeDecision(ctx SupervisionContext) (decision SupervisionDecision, reason string)
}

// SupervisionStrategyDecisionMakerFN 定义了监督策略决策器函数类型，用于根据当前监督上下文做出决策。
type SupervisionStrategyDecisionMakerFN func(ctx SupervisionContext) SupervisionDecision

// MakeDecision 实现 SupervisionStrategyDecisionMaker 接口。
func (maker SupervisionStrategyDecisionMakerFN) MakeDecision(ctx SupervisionContext) SupervisionDecision {
	return maker(ctx)
}

// SupervisionStrategy 定义了监督策略接口，用于在监督过程中选择不同的处理方式。
type SupervisionStrategy interface {
	// Supervise 方法用于在监督过程中选择不同的处理方式。
	// 返回需要处理的 ActorRefs 和监督决策。
	Supervise(ctx SupervisionContext) (targets ActorRefs, decision SupervisionDecision, reason string)
}

// SupervisionContext 定义了监督上下文接口，用于在监督过程中传递上下文信息。
type SupervisionContext interface {
	// Logger 方法用于获取当前监督上下文的日志记录器。
	Logger() log.Logger

	// Child 方法用于获取仅包含触发当前监督上下文的 ActorRefs。
	Child() ActorRefs

	// Children 方法用于获取当前监督上下文的所有子 ActorRefs。
	Children() ActorRefs

	// Message 获取导致当前监督上下文触发的消息。
	Message() Message

	// 获取故障信息。
	Fault() Message

	// FaultStack 获取故障堆栈。
	FaultStack() []byte
}

type OneForOneStrategyOption func(options *OneForOneStrategyOptions)

type OneForOneStrategyOptions struct {
	InitialDelay time.Duration // 初始延迟时间，默认 1 秒
	MaxDelay     time.Duration // 最大延迟时间，默认 1 分钟
	Factor       float64       // 退避因子，默认 2.0
	Jitter       bool          // 是否启用抖动，默认 true
}

// WithOneForOneStrategyOptions 返回一个设置 OneForOneStrategyOptions 的配置项。
//
// opts 为 OneForOneStrategyOptions 结构体。
//
// 返回:
//   - OneForOneStrategyOption: 一个设置 OneForOneStrategyOptions 的配置项。
func WithOneForOneStrategyOptions(opts OneForOneStrategyOptions) OneForOneStrategyOption {
	return func(options *OneForOneStrategyOptions) {
		*options = opts
	}
}

// WithOneForOneStrategyInitialDelay 返回一个设置 OneForOneStrategyOptions.InitialDelay 的配置项。
//
// initialDelay 为初始延迟时间。初始延迟时间是用于第一次重试的延迟时间。
// 如果 initialDelay <= 0，则使用默认值 1 秒。
//
// 返回:
//   - OneForOneStrategyOption: 一个设置 OneForOneStrategyOptions.InitialDelay 的配置项。如果 initialDelay <= 0，则使用默认值 1 秒。
func WithOneForOneStrategyInitialDelay(initialDelay time.Duration) OneForOneStrategyOption {
	return func(options *OneForOneStrategyOptions) {
		if initialDelay > 0 {
			options.InitialDelay = initialDelay
		}
	}
}

// WithOneForOneStrategyMaxDelay 返回一个设置 OneForOneStrategyOptions.MaxDelay 的配置项。
//
// maxDelay 为最大延迟时间。最大延迟时间是用于最大重试的延迟时间。
// 如果 maxDelay <= 0，则使用默认值 1 分钟。
//
// 返回:
//   - OneForOneStrategyOption: 一个设置 OneForOneStrategyOptions.MaxDelay 的配置项。如果 maxDelay <= 0，则使用默认值 1 分钟。
func WithOneForOneStrategyMaxDelay(maxDelay time.Duration) OneForOneStrategyOption {
	return func(options *OneForOneStrategyOptions) {
		if maxDelay > 0 {
			options.MaxDelay = maxDelay
		}
	}
}

// WithOneForOneStrategyFactor 返回一个设置 OneForOneStrategyOptions.Factor 的配置项。
//
// factor 为退避因子。退避因子是用于计算重试延迟的因子。
// 如果 factor <= 0，则使用默认值 2.0。
//
// 返回:
//   - OneForOneStrategyOption: 一个设置 OneForOneStrategyOptions.Factor 的配置项。如果 factor <= 0，则使用默认值 2.0。
func WithOneForOneStrategyFactor(factor float64) OneForOneStrategyOption {
	return func(options *OneForOneStrategyOptions) {
		if factor > 0 {
			options.Factor = factor
		}
	}
}

// WithOneForOneStrategyJitter 返回一个设置 OneForOneStrategyOptions.Jitter 的配置项。
//
// jitter 为是否启用抖动。抖动是用于在重试时添加随机抖动。
// 如果 jitter == false，则使用默认值 true。
//
// 返回:
//   - OneForOneStrategyOption: 一个设置 OneForOneStrategyOptions.Jitter 的配置项。如果 jitter == false，则使用默认值 true。
func WithOneForOneStrategyJitter(jitter bool) OneForOneStrategyOption {
	return func(options *OneForOneStrategyOptions) {
		if jitter {
			options.Jitter = jitter
		}
	}
}

// OneForOneStrategy 返回一个一对一监督策略，使用指数退避算法。
// 它将根据决策器返回的决策决定如何处理故障的 Actor。
//
// 返回:
//   - SupervisionStrategy: 一个一对一监督策略。
func OneForOneStrategy(decisionMaker SupervisionStrategyDecisionMaker, options ...OneForOneStrategyOption) SupervisionStrategy {
	var opts = &OneForOneStrategyOptions{
		InitialDelay: time.Second,
		MaxDelay:     time.Minute,
		Factor:       2.0,
		Jitter:       true,
	}

	for _, option := range options {
		option(opts)
	}

	return &oneForOneStrategy{
		backoff:       utils.NewExponentialBackoff(opts.InitialDelay, opts.MaxDelay, opts.Factor, opts.Jitter),
		decisionMaker: decisionMaker,
	}
}

type oneForOneStrategy struct {
	backoff       *utils.ExponentialBackoff
	decisionMaker SupervisionStrategyDecisionMaker
}

func (strategy *oneForOneStrategy) Supervise(ctx SupervisionContext) (targets ActorRefs, decision SupervisionDecision, reason string) {
	decision, reason = strategy.decisionMaker.MakeDecision(ctx)
	return ctx.Child(), decision, reason
}

type OneForAllStrategyOption func(options *OneForAllStrategyOptions)

type OneForAllStrategyOptions struct {
	InitialDelay time.Duration // 初始延迟时间，默认 1 秒
	MaxDelay     time.Duration // 最大延迟时间，默认 1 分钟
	Factor       float64       // 退避因子，默认 2.0
	Jitter       bool          // 是否启用抖动，默认 true
}

// WithOneForAllStrategyOptions 返回一个设置 OneForAllStrategyOptions 的配置项。
//
// opts 为 OneForAllStrategyOptions 结构体。
//
// 返回:
//   - OneForAllStrategyOption: 一个设置 OneForAllStrategyOptions 的配置项。
func WithOneForAllStrategyOptions(opts OneForAllStrategyOptions) OneForAllStrategyOption {
	return func(options *OneForAllStrategyOptions) {
		*options = opts
	}
}

// WithOneForAllStrategyInitialDelay 返回一个设置 OneForAllStrategyOptions.InitialDelay 的配置项。
//
// initialDelay 为初始延迟时间。初始延迟时间是用于第一次重试的延迟时间。
// 如果 initialDelay <= 0，则使用默认值 1 秒。
//
// 返回:
//   - OneForAllStrategyOption: 一个设置 OneForAllStrategyOptions.InitialDelay 的配置项。如果 initialDelay <= 0，则使用默认值 1 秒。
func WithOneForAllStrategyInitialDelay(initialDelay time.Duration) OneForAllStrategyOption {
	return func(options *OneForAllStrategyOptions) {
		if initialDelay > 0 {
			options.InitialDelay = initialDelay
		}
	}
}

// WithOneForAllStrategyMaxDelay 返回一个设置 OneForAllStrategyOptions.MaxDelay 的配置项。
//
// maxDelay 为最大延迟时间。最大延迟时间是用于最大重试的延迟时间。
// 如果 maxDelay <= 0，则使用默认值 1 分钟。
//
// 返回:
//   - OneForAllStrategyOption: 一个设置 OneForAllStrategyOptions.MaxDelay 的配置项。如果 maxDelay <= 0，则使用默认值 1 分钟。
func WithOneForAllStrategyMaxDelay(maxDelay time.Duration) OneForAllStrategyOption {
	return func(options *OneForAllStrategyOptions) {
		if maxDelay > 0 {
			options.MaxDelay = maxDelay
		}
	}
}

// WithOneForAllStrategyFactor 返回一个设置 OneForAllStrategyOptions.Factor 的配置项。
//
// factor 为退避因子。退避因子是用于计算重试延迟的因子。
// 如果 factor <= 0，则使用默认值 2.0。
//
// 返回:
//   - OneForAllStrategyOption: 一个设置 OneForAllStrategyOptions.Factor 的配置项。如果 factor <= 0，则使用默认值 2.0。
func WithOneForAllStrategyFactor(factor float64) OneForAllStrategyOption {
	return func(options *OneForAllStrategyOptions) {
		if factor > 0 {
			options.Factor = factor
		}
	}
}

// WithOneForAllStrategyJitter 返回一个设置 OneForAllStrategyOptions.Jitter 的配置项。
//
// jitter 为是否启用抖动。抖动是用于在重试时添加随机抖动。
// 如果 jitter == false，则使用默认值 true。
//
// 返回:
//   - OneForAllStrategyOption: 一个设置 OneForAllStrategyOptions.Jitter 的配置项。如果 jitter == false，则使用默认值 true。
func WithOneForAllStrategyJitter(jitter bool) OneForAllStrategyOption {
	return func(options *OneForAllStrategyOptions) {
		if jitter {
			options.Jitter = jitter
		}
	}
}

// OneForAllStrategy 返回一个一对多监督策略，使用指数退避算法。
// 它将根据决策器返回的决策决定如何处理故障的 Actor。
//
// 返回:
//   - SupervisionStrategy: 一个一对多监督策略。
func OneForAllStrategy(decisionMaker SupervisionStrategyDecisionMaker, options ...OneForAllStrategyOption) SupervisionStrategy {
	var opts = &OneForAllStrategyOptions{
		InitialDelay: time.Second,
		MaxDelay:     time.Minute,
		Factor:       2.0,
		Jitter:       true,
	}

	for _, option := range options {
		option(opts)
	}

	return &oneForAllStrategy{
		backoff:       utils.NewExponentialBackoff(opts.InitialDelay, opts.MaxDelay, opts.Factor, opts.Jitter),
		decisionMaker: decisionMaker,
	}
}

type oneForAllStrategy struct {
	backoff       *utils.ExponentialBackoff
	decisionMaker SupervisionStrategyDecisionMaker
}

func (strategy *oneForAllStrategy) Supervise(ctx SupervisionContext) (targets ActorRefs, decision SupervisionDecision, reason string) {
	decision, reason = strategy.decisionMaker.MakeDecision(ctx)
	return ctx.Children(), decision, reason
}
