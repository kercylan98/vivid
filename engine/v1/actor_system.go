package vivid

import (
	"sync"
	"sync/atomic"

	"github.com/kercylan98/vivid/engine/v1/metrics"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/engine/v1/internal/processor"
)

// ActorSystem 定义了 Actor 系统的核心接口。
//
// ActorSystem 是 vivid 框架的入口点，负责管理所有 Actor 的生命周期。
// 它继承了 ActorContext 接口，本身也是一个特殊的根 Actor。
//
// 主要职责：
//   - 管理 Actor 的创建、启动和停止
//   - 提供系统级别的配置和服务
//   - 处理 Actor 之间的消息路由
//   - 监控系统运行状态
type ActorSystem interface {
	ActorContext

	// Logger 获取 Actor 系统的日志记录器。
	//
	// 返回系统级别的日志记录器，用于记录系统运行状态和错误信息。
	Logger() log.Logger

	// Shutdown 关闭 Actor 系统。
	//
	// 参数：
	//   - poison: 是否使用优雅关闭方式
	//     - true: 优雅关闭，等待 Actor 处理完当前消息后停止
	//     - false: 立即关闭，强制停止所有 Actor
	//   - reason: 关闭的原因说明，用于日志记录和调试
	//
	// 返回关闭过程中可能出现的错误。
	Shutdown(poison bool, reason ...string) error
}

// NewActorSystemFromConfig 根据配置创建一个新的 Actor 系统。
//
// 该函数会根据配置启用相应的功能模块：
//   - 指标收集系统
//   - 钩子系统
//   - 日志记录
//   - Actor 注册表
//
// 参数 config 包含了 Actor 系统的完整配置信息。
// 返回一个完全初始化的 ActorSystem 实例。
func NewActorSystemFromConfig(config *ActorSystemConfiguration) ActorSystem {
	sys := &actorSystem{
		config: *config,
	}

	if config.Metrics {
		sys.metrics = newActorSystemMetrics(metrics.NewManagerWithConfigurators(metrics.ManagerConfiguratorFN(func(c *metrics.ManagerConfiguration) {
			c.WithLogger(config.Logger.WithGroup("metrics"))
		})))
		config.Hooks = append(sys.metrics.hooks(), config.Hooks...)
	}

	if len(config.Hooks) > 0 {
		sys.hooks = newHookRegister(config.Hooks)
		sys.config.Hooks = nil
	}

	daemon := new(daemonActor)

	sys.registry = processor.NewRegistryWithConfigurators(processor.RegistryConfiguratorFN(func(c *processor.RegistryConfiguration) {
		c.WithLogger(config.Logger.WithGroup("unit-registry"))
		c.WithDaemon(daemon)
	}))

	ctx := newActorContext(sys, sys.registry.GetUnitIdentifier(), nil, ActorProviderFN(func() Actor {
		return daemon
	}), NewActorConfiguration(func(configuration *ActorConfiguration) {
		configuration.WithSupervisionProvider(SupervisorProviderFN(func() Supervisor {
			return StandardSupervisorWithConfigurators()
		}))
	}))

	bindActorContext(sys, nil, ctx)
	sys.ActorContext = ctx

	return sys
}

// NewActorSystemWithOptions 使用选项模式创建一个新的 Actor 系统。
//
// 这是创建 Actor 系统的推荐方式，提供了灵活的配置选项。
//
// 参数 options 是一系列配置选项函数，用于自定义系统行为。
// 返回一个配置完成的 ActorSystem 实例。
func NewActorSystemWithOptions(options ...ActorSystemOption) ActorSystem {
	config := NewActorSystemConfiguration(options...)
	return NewActorSystemFromConfig(config)
}

// NewActorSystemWithConfigurators 使用配置器模式创建一个新的 Actor 系统。
//
// 配置器模式提供了更高级的配置方式，适合复杂的系统配置需求。
//
// 参数 configurators 是一系列配置器，用于修改系统配置。
// 返回一个配置完成的 ActorSystem 实例。
func NewActorSystemWithConfigurators(configurators ...ActorSystemConfigurator) ActorSystem {
	config := NewActorSystemConfiguration()
	for _, c := range configurators {
		c.Configure(config)
	}
	return NewActorSystemFromConfig(config)
}

type actorSystem struct {
	ActorContext
	config     ActorSystemConfiguration
	registry   processor.Registry
	shutdownWG sync.WaitGroup
	futureGuid atomic.Uint64
	hooks      *hookRegister
	metrics    *actorSystemMetrics
}

func (sys *actorSystem) Logger() log.Logger {
	return sys.config.Logger
}

func (sys *actorSystem) Shutdown(poison bool, reason ...string) error {
	if poison {
		sys.PoisonKill(sys.Ref(), reason...)
	} else {
		sys.Kill(sys.Ref(), reason...)
	}
	sys.shutdownWG.Wait()
	return sys.registry.Shutdown()
}
