package vivid

import (
	"math"
	"time"

	"github.com/kercylan98/go-log/log"
)

// StandardSupervisorFromConfig 根据配置创建标准监管者。
//
// 标准监管者提供了常用的监管策略，包括退避重启和延迟处理等功能。
// 参数 config 包含了监管者的完整配置信息。
// 返回一个配置完成的 Supervisor 实例。
func StandardSupervisorFromConfig(config *StandardSupervisorConfiguration) Supervisor {
	return &standardSupervisor{
		config: *config,
	}
}

// StandardSupervisorWithConfigurators 使用配置器模式创建标准监管者。
//
// 配置器模式提供了灵活的配置方式，适合复杂的监管策略配置。
// 参数 configurators 是一系列配置器函数。
// 返回一个配置完成的 Supervisor 实例。
func StandardSupervisorWithConfigurators(configurators ...StandardSupervisorConfigurator) Supervisor {
	config := NewStandardSupervisorConfiguration()
	for _, c := range configurators {
		c.Configure(config)
	}
	return StandardSupervisorFromConfig(config)
}

// StandardSupervisorWithOptions 使用选项模式创建标准监管者。
//
// 选项模式是推荐的配置方式，提供了类型安全的配置选项。
// 参数 options 是一系列配置选项函数。
// 返回一个配置完成的 Supervisor 实例。
func StandardSupervisorWithOptions(options ...StandardSupervisorOption) Supervisor {
	config := NewStandardSupervisorConfiguration(options...)
	return StandardSupervisorFromConfig(config)
}

type standardSupervisor struct {
	config StandardSupervisorConfiguration
}

func (s *standardSupervisor) Strategy(fatal *Fatal) SupervisorDirective {
	var directive = DirectiveRestart
	if s.config.DirectiveProvider != nil {
		directive = s.config.DirectiveProvider.Provide(fatal)
	}

	// 如果启用了延迟返回指令，先进行延迟
	if s.config.DelayEnabled && s.config.DelayDuration > 0 {
		time.Sleep(s.config.DelayDuration)
	}

	// 如果不是重启指令或未启用退避，直接返回
	if directive != DirectiveRestart || !s.config.BackoffEnabled {
		return directive
	}

	// 处理退避重启逻辑
	currentCount := fatal.RestartCount()

	// 检查是否超过最大重试次数
	if currentCount >= s.config.BackoffMaxRetries {
		fatal.ctx.Logger().Error("supervisor", log.Int("backoff-restart", currentCount), log.Int("limit", s.config.BackoffMaxRetries), log.Any("reason", fatal.reason))
		return DirectiveKill
	}

	// 计算退避延迟时间
	if currentCount > 0 {
		delay := s.calculateBackoffDelay(currentCount)
		fatal.ctx.Logger().Warn("supervisor", log.Int("backoff-restart", currentCount), log.Int("limit", s.config.BackoffMaxRetries), log.Duration("delay", delay), log.Any("reason", fatal.reason))
		time.Sleep(delay)
	}

	return DirectiveRestart
}

// calculateBackoffDelay 计算退避延迟时间。
//
// 使用指数退避算法计算延迟时间，公式为：
// delay = initialDelay * (multiplier ^ retryCount)
// 结果会被限制在最大延迟时间内。
func (s *standardSupervisor) calculateBackoffDelay(retryCount int) time.Duration {
	// 使用指数退避算法：delay = initialDelay * (multiplier ^ retryCount)
	delayFloat := float64(s.config.BackoffInitialDelay) * math.Pow(s.config.BackoffMultiplier, float64(retryCount))
	delay := time.Duration(delayFloat)

	// 限制在最大延迟时间内
	if delay > s.config.BackoffMaxDelay {
		delay = s.config.BackoffMaxDelay
	}

	return delay
}
