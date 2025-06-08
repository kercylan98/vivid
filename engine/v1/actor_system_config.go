package vivid

import (
	"time"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/configurator"
)

// NewActorSystemConfiguration 创建新的ActorSystem配置实例
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *ActorSystemConfiguration: 配置实例
func NewActorSystemConfiguration(options ...ActorSystemOption) *ActorSystemConfiguration {
	c := &ActorSystemConfiguration{
		Logger: log.GetBuilder().FromConfigurators(log.LoggerConfiguratorFn(func(config log.LoggerConfiguration) {
			config.
				WithLeveler(log.LevelInfo).
				WithEnableColor(true).
				WithErrTrackLevel(log.LevelError).
				WithTrackBeautify(true).
				WithMessageFormatter(func(message string) string {
					return message
				})
		})),
		FutureDefaultTimeout: time.Second,
	}
	for _, option := range options {
		option(c)
	}
	return c
}

type (
	// ActorSystemConfigurator 配置器接口
	ActorSystemConfigurator = configurator.Configurator[*ActorSystemConfiguration]

	// ActorSystemConfiguratorFN 配置器函数类型
	ActorSystemConfiguratorFN = configurator.FN[*ActorSystemConfiguration]

	// ActorSystemOption 配置选项函数类型
	ActorSystemOption = configurator.Option[*ActorSystemConfiguration]

	// ActorSystemConfiguration ActorSystem配置结构体
	//
	// 所有字段均为私有，通过 GetXxx 方法获取值，通过 WithXxx 方法设置值。
	ActorSystemConfiguration struct {
		Logger               log.Logger
		FutureDefaultTimeout time.Duration
		Hooks                []Hook
		Metrics              bool // 是否启用指标采集
	}
)

// WithLogger 设置日志器
func (c *ActorSystemConfiguration) WithLogger(logger log.Logger) *ActorSystemConfiguration {
	c.Logger = logger
	return c
}

// WithActorSystemLogger 设置 ActorSystem 日志器
func WithActorSystemLogger(logger log.Logger) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.Logger = logger
	}
}

// WithFutureDefaultTimeout 设置 Future 默认超时时间
func (c *ActorSystemConfiguration) WithFutureDefaultTimeout(timeout time.Duration) *ActorSystemConfiguration {
	c.FutureDefaultTimeout = timeout
	return c
}

// WithActorSystemFutureDefaultTimeoutFn 设置 Future 默认超时时间
func WithActorSystemFutureDefaultTimeoutFn(timeout time.Duration) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.FutureDefaultTimeout = timeout
	}
}

// WithHooks 设置 ActorSystem 钩子
func (c *ActorSystemConfiguration) WithHooks(hooks ...Hook) *ActorSystemConfiguration {
	c.Hooks = append(c.Hooks, hooks...)
	return c
}

// WithActorSystemHooks 设置 ActorSystem 钩子
func WithActorSystemHooks(hooks ...Hook) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.Hooks = append(c.Hooks, hooks...)
	}
}

// WithHookProviders 设置 ActorSystem 钩子提供者
func (c *ActorSystemConfiguration) WithHookProviders(hookProviders ...HookProvider) *ActorSystemConfiguration {
	for _, hookProvider := range hookProviders {
		c.Hooks = append(c.Hooks, hookProvider.hooks()...)
	}
	return c
}

// WithActorSystemHookProviders 设置 ActorSystem 钩子提供者
func WithActorSystemHookProviders(hookProviders ...HookProvider) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.WithHookProviders(hookProviders...)
	}
}

// WithMetrics 设置 ActorSystem 是否启用指标采集
func (c *ActorSystemConfiguration) WithMetrics(enable bool) *ActorSystemConfiguration {
	c.Metrics = enable
	return c
}
