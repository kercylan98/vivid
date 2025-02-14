package vivid

import "github.com/kercylan98/go-log/log"

var (
	_                  ActorSystem        = (*actorSystem)(nil) // 确保 actorSystem 实现了 ActorSystem 接口
	actorSystemBuilder ActorSystemBuilder                       // ActorSystem 的全局构建器
)

// NewActorSystem 该函数是综合了 ActorSystemBuilder 的快捷创建方法
//   - 如果不传入任何配置器，则会使用默认配置创建 ActorSystem 实例
//   - 如果传入配置器，则会使用配置器创建 ActorSystem 实例
func NewActorSystem(configurator ...ActorSystemConfigurator) ActorSystem {
	builder := GetActorSystemBuilder()
	if len(configurator) > 0 {
		return builder.FromConfigurators(configurator...)
	}
	return builder.Build()
}

// GetActorSystemBuilder 返回 ActorSystem 的构建器
func GetActorSystemBuilder() ActorSystemBuilder {
	return actorSystemBuilder
}

// ActorSystemBuilder 是 ActorSystem 的构建器
type ActorSystemBuilder struct{}

// Build 用于构建 ActorSystem 实例
func (builder ActorSystemBuilder) Build() ActorSystem {
	sys := &actorSystem{}
	config := NewActorSystemConfig().InitDefault()
	processManager := newProcessManager("localhost", config.FetchCodec(), log.ProviderFn(func() log.Logger {
		return config.FetchLogger()
	}))
	sys.actorSystemInternal = newActorSystemInternal(sys, config, processManager)
	return sys
}

// FromConfiguration 通过配置构建 ActorSystem 实例
func (builder ActorSystemBuilder) FromConfiguration(config ActorSystemConfiguration) ActorSystem {
	config.InitDefault()

	processManager := newProcessManager("localhost", config.FetchCodec(), log.ProviderFn(func() log.Logger {
		return config.FetchLogger()
	}))

	sys := &actorSystem{}
	sys.actorSystemInternal = newActorSystemInternal(sys, config, processManager)
	sys.setConfig(config)

	return sys
}

// FromConfigurators 通过配置器构建 ActorSystem 实例
func (builder ActorSystemBuilder) FromConfigurators(configurators ...ActorSystemConfigurator) ActorSystem {
	var config = NewActorSystemConfig()
	for _, c := range configurators {
		c.Configure(config)
	}
	return builder.FromConfiguration(config)
}

// FromCustomize 通过自定义配置构建 ActorSystem 实例
func (builder ActorSystemBuilder) FromCustomize(configuration ActorSystemConfiguration, configurators ...ActorSystemConfigurator) ActorSystem {
	for _, configurator := range configurators {
		configurator.Configure(configuration)
	}
	return builder.FromConfiguration(configuration)
}

// ActorSystem 是完整的 Actor 系统的接口，它包含了对于 Actor Model 的完整实现。
//   - Actor 系统是基于 Actor 模式的并发编程模型，负责管理和调度 Actor 实例。
//   - 它提供了创建、监控、发送消息、以及终止 Actor 的功能。
//   - 在 Actor 系统中，所有的操作都是通过消息传递的方式进行的，
//   - 其中每个 Actor 都是独立的计算单元，通过收发消息与其他 Actor 进行交互。
//
// Actor 系统的设计遵循了高并发和低耦合的原则，能够有效地处理大量并发任务，
// 同时避免传统线程模型中的共享状态问题和锁竞争问题。
// 这使得 Actor 系统在需要高并发、分布式计算和容错的场景中非常适用。
type ActorSystem interface {
	actorSystemInternal
	ActorContextSpawner
	ActorContextLife
	ActorContextLogger
	ActorContextActions

	// Start 启动 Actor 系统
	Start() error

	// StartP 启动 Actor 系统，并在发生异常时 panic
	StartP() ActorSystem

	// Shutdown 关闭 Actor 系统
	Shutdown() error
}

type actorSystem struct {
	actorSystemInternal
	ActorContextSpawner
	ActorContextLife
	ActorContextLogger
	ActorContextActions
	daemon ActorContext // 根 Actor
}

// Start 启动 Actor 系统
func (sys *actorSystem) Start() error {
	// 初始化 Root Actor
	daemon := generateRootActorContext(sys, ActorProviderFn(func() Actor {
		return new(rootActor)
	}), ActorConfiguratorFn(func(config ActorConfiguration) {
		config.WithLoggerProvider(sys.getConfig().FetchLoggerProvider())
	}))
	sys.daemon = daemon
	sys.ActorContextSpawner = daemon
	sys.ActorContextLife = daemon
	sys.ActorContextLogger = daemon
	sys.ActorContextActions = daemon
	sys.getProcessManager().setDaemon(daemon.getProcess())

	return nil
}

// StartP 启动 Actor 系统，并在发生异常时 panic
func (sys *actorSystem) StartP() ActorSystem {
	if err := sys.Start(); err != nil {
		panic(err)
	}
	return sys
}

// Shutdown 关闭 Actor 系统
func (sys *actorSystem) Shutdown() error {
	// 设置关闭监听
	var wait = make(chan struct{})
	if err := sys.daemon.Watch(sys.daemon.Ref(), WatchHandlerFn(func(ctx ActorContext, stopped OnWatchStopped) {
		close(wait)
	})); err != nil {
		return err
	}

	// 优雅关闭根 Actor
	sys.daemon.PoisonKill(sys.daemon.Ref())

	<-wait
	return nil
}
