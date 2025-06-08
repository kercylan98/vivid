package vivid

import (
	"github.com/kercylan98/vivid/engine/v1/metrics"
	"sync"
	"sync/atomic"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/engine/v1/internal/processor"
)

type ActorSystem interface {
	ActorContext

	// Logger 获取日志记录器。
	Logger() log.Logger

	// Shutdown 关闭 Actor 系统。
	// 会终止所有 Actor 并释放相关资源。
	Shutdown(poison bool, reason ...string) error
}

func NewActorSystemFromConfig(config *ActorSystemConfiguration) ActorSystem {
	sys := &actorSystem{
		config: *config,
		registry: processor.NewRegistryWithConfigurators(processor.RegistryConfiguratorFN(func(c *processor.RegistryConfiguration) {
			c.WithLogger(config.Logger.WithGroup("unit-registry"))
		})),
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

	ctx := newActorContext(sys, sys.registry.GetUnitIdentifier(), nil, ActorProviderFN(func() Actor {
		return ActorFN(func(context ActorContext) {})
	}), NewActorConfiguration())
	bindActorContext(sys, nil, ctx)
	sys.ActorContext = ctx

	return sys
}

func NewActorSystemWithOptions(options ...ActorSystemOption) ActorSystem {
	config := NewActorSystemConfiguration(options...)
	return NewActorSystemFromConfig(config)
}

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
