package vivid

import (
    "github.com/kercylan98/go-log/log"
    "math"
    "time"
)

func StandardSupervisorFromConfig(config *StandardSupervisorConfiguration) Supervisor {
    return &standardSupervisor{
        config: *config,
    }
}

func StandardSupervisorWithConfigurators(configurators ...StandardSupervisorConfigurator) Supervisor {
    config := NewStandardSupervisorConfiguration()
    for _, c := range configurators {
        c.Configure(config)
    }
    return StandardSupervisorFromConfig(config)
}

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

// calculateBackoffDelay 计算退避延迟时间
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
