package vivid

import (
    "time"

    "github.com/kercylan98/vivid/src/configurator"
)

// NewStandardSupervisorConfiguration 创建新的StandardSupervisor配置实例
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *StandardSupervisorConfiguration: 配置实例
func NewStandardSupervisorConfiguration(options ...StandardSupervisorOption) *StandardSupervisorConfiguration {
    c := &StandardSupervisorConfiguration{
        // 设置默认值
        BackoffEnabled:      true,
        BackoffInitialDelay: time.Millisecond * 100,
        BackoffMaxDelay:     time.Minute * 5,
        BackoffMultiplier:   2.0,
        BackoffMaxRetries:   20,
        DelayEnabled:        false,
        DelayDuration:       time.Millisecond * 500,
    }
    for _, option := range options {
        option(c)
    }
    return c
}

type (
    // StandardSupervisorConfigurator 配置器接口
    StandardSupervisorConfigurator = configurator.Configurator[*StandardSupervisorConfiguration]

    // StandardSupervisorConfiguratorFN 配置器函数类型
    StandardSupervisorConfiguratorFN = configurator.FN[*StandardSupervisorConfiguration]

    // StandardSupervisorOption 配置选项函数类型
    StandardSupervisorOption = configurator.Option[*StandardSupervisorConfiguration]

    // StandardSupervisorConfiguration StandardSupervisor配置结构体
    //
    // 所有字段均为私有，通过 GetXxx 方法获取值，通过 WithXxx 方法设置值。
    StandardSupervisorConfiguration struct {
        DirectiveProvider SupervisorDirectiveProvider
        // 退避重启相关配置
        BackoffEnabled      bool          // 是否启用退避重启
        BackoffInitialDelay time.Duration // 初始延迟时间
        BackoffMaxDelay     time.Duration // 最大延迟时间
        BackoffMultiplier   float64       // 延迟倍增因子
        BackoffMaxRetries   int           // 最大重试次数
        // 延迟返回指令相关配置
        DelayEnabled  bool          // 是否启用延迟返回指令
        DelayDuration time.Duration // 延迟持续时间
    }
)

// WithDirectiveProvider 设置 StandardSupervisorConfiguration 的 DirectiveProvider 字段
func (c *StandardSupervisorConfiguration) WithDirectiveProvider(directiveProvider SupervisorDirectiveProvider) *StandardSupervisorConfiguration {
    c.DirectiveProvider = directiveProvider
    return c
}

// WithStandardSupervisorDirectiveProvider 设置 StandardSupervisorConfiguration 的 DirectiveProvider 字段
func WithStandardSupervisorDirectiveProvider(directiveProvider SupervisorDirectiveProvider) StandardSupervisorOption {
    return func(c *StandardSupervisorConfiguration) {
        c.WithDirectiveProvider(directiveProvider)
    }
}

// WithBackoffEnabled 设置是否启用退避重启
func (c *StandardSupervisorConfiguration) WithBackoffEnabled(enabled bool) *StandardSupervisorConfiguration {
    c.BackoffEnabled = enabled
    return c
}

// WithStandardSupervisorBackoffEnabled 设置是否启用退避重启的选项函数
func WithStandardSupervisorBackoffEnabled(enabled bool) StandardSupervisorOption {
    return func(c *StandardSupervisorConfiguration) {
        c.WithBackoffEnabled(enabled)
    }
}

// WithBackoffInitialDelay 设置退避重启的初始延迟时间
func (c *StandardSupervisorConfiguration) WithBackoffInitialDelay(delay time.Duration) *StandardSupervisorConfiguration {
    c.BackoffInitialDelay = delay
    return c
}

// WithStandardSupervisorBackoffInitialDelay 设置退避重启初始延迟时间的选项函数
func WithStandardSupervisorBackoffInitialDelay(delay time.Duration) StandardSupervisorOption {
    return func(c *StandardSupervisorConfiguration) {
        c.WithBackoffInitialDelay(delay)
    }
}

// WithBackoffMaxDelay 设置退避重启的最大延迟时间
func (c *StandardSupervisorConfiguration) WithBackoffMaxDelay(delay time.Duration) *StandardSupervisorConfiguration {
    c.BackoffMaxDelay = delay
    return c
}

// WithStandardSupervisorBackoffMaxDelay 设置退避重启最大延迟时间的选项函数
func WithStandardSupervisorBackoffMaxDelay(delay time.Duration) StandardSupervisorOption {
    return func(c *StandardSupervisorConfiguration) {
        c.WithBackoffMaxDelay(delay)
    }
}

// WithBackoffMultiplier 设置退避重启的延迟倍增因子
func (c *StandardSupervisorConfiguration) WithBackoffMultiplier(multiplier float64) *StandardSupervisorConfiguration {
    c.BackoffMultiplier = multiplier
    return c
}

// WithStandardSupervisorBackoffMultiplier 设置退避重启延迟倍增因子的选项函数
func WithStandardSupervisorBackoffMultiplier(multiplier float64) StandardSupervisorOption {
    return func(c *StandardSupervisorConfiguration) {
        c.WithBackoffMultiplier(multiplier)
    }
}

// WithBackoffMaxRetries 设置退避重启的最大重试次数
func (c *StandardSupervisorConfiguration) WithBackoffMaxRetries(maxRetries int) *StandardSupervisorConfiguration {
    c.BackoffMaxRetries = maxRetries
    return c
}

// WithStandardSupervisorBackoffMaxRetries 设置退避重启最大重试次数的选项函数
func WithStandardSupervisorBackoffMaxRetries(maxRetries int) StandardSupervisorOption {
    return func(c *StandardSupervisorConfiguration) {
        c.WithBackoffMaxRetries(maxRetries)
    }
}

// WithDelayEnabled 设置是否启用延迟返回指令
func (c *StandardSupervisorConfiguration) WithDelayEnabled(enabled bool) *StandardSupervisorConfiguration {
    c.DelayEnabled = enabled
    return c
}

// WithStandardSupervisorDelayEnabled 设置是否启用延迟返回指令的选项函数
func WithStandardSupervisorDelayEnabled(enabled bool) StandardSupervisorOption {
    return func(c *StandardSupervisorConfiguration) {
        c.WithDelayEnabled(enabled)
    }
}

// WithDelayDuration 设置延迟返回指令的延迟持续时间
func (c *StandardSupervisorConfiguration) WithDelayDuration(duration time.Duration) *StandardSupervisorConfiguration {
    c.DelayDuration = duration
    return c
}

// WithStandardSupervisorDelayDuration 设置延迟返回指令延迟持续时间的选项函数
func WithStandardSupervisorDelayDuration(duration time.Duration) StandardSupervisorOption {
    return func(c *StandardSupervisorConfiguration) {
        c.WithDelayDuration(duration)
    }
}
