package vivid

import (
	"fmt"
	"github.com/kercylan98/go-log/log"
)

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

	sys.actorSystemInternal = newActorSystemInternal(sys, config)
	return sys
}

// FromConfiguration 通过配置构建 ActorSystem 实例
func (builder ActorSystemBuilder) FromConfiguration(config ActorSystemConfiguration) ActorSystem {
	config.InitDefault()
	sys := &actorSystem{}
	sys.actorSystemInternal = newActorSystemInternal(sys, config)
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
	ActorContextTransportInteractive

	// Start 启动 Actor 系统
	Start() error

	// StartP 启动 Actor 系统，并在发生异常时 panic
	StartP() ActorSystem

	// Shutdown 关闭 Actor 系统
	Shutdown() error

	// ShutdownP 关闭 Actor 系统，并在发生异常时 panic
	ShutdownP() ActorSystem
}

type actorSystem struct {
	actorSystemInternal
	ActorContextSpawner
	ActorContextLife
	ActorContextLogger
	ActorContextTransportInteractive
	daemon ActorContext // 根 Actor
}

func (sys *actorSystem) Logger() log.Logger {
	return sys.getConfig().FetchLogger()
}

// Start 启动 Actor 系统
func (sys *actorSystem) Start() error {
	sys.startingLog(log.String("stage", "start"))

	// 初始化远程通信
	if err := sys.actorSystemInternal.initRemote(); err != nil {
		return err
	}

	// 初始化进程控制器
	sys.actorSystemInternal.initProcessManager()

	// 初始化 Root Actor
	daemon := generateRootActorContext(sys, ActorProviderFn(func() Actor {
		return new(guardActor)
	}), ActorConfiguratorFn(func(config ActorConfiguration) {
		config.WithLoggerProvider(sys.getConfig().FetchLoggerProvider())
		config.WithSupervisor(getDefaultSupervisor(sys.getConfig().FetchDefaultSupervisorRestartLimit()))
	}))
	sys.daemon = daemon
	sys.ActorContextSpawner = daemon
	sys.ActorContextLife = daemon
	sys.ActorContextLogger = daemon
	sys.ActorContextTransportInteractive = daemon
	sys.getProcessManager().setDaemon(daemon.getProcess())

	// 相关日志
	remoteEnabled := sys.getProcessManager().getHost() != sys.getConfig().FetchName()
	if !remoteEnabled {
		sys.startingLog(log.String("stage", "remote"), log.Bool("enabled", remoteEnabled), log.String("listen", "unused"), log.String("info", "remote function only supports active access"))
	} else {
		sys.startingLog(log.String("stage", "remote"), log.Bool("enabled", remoteEnabled), log.String("listen", sys.getProcessManager().getHost()))
	}
	sys.startingLog(log.String("stage", "started"))
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
	sys.shutdownLog(log.String("stage", "shutdown"))

	// 设置关闭监听
	var wait = make(chan struct{})
	if err := sys.daemon.Watch(sys.daemon.Ref(), WatchHandlerFn(func(ctx ActorContext, stopped OnWatchStopped) {
		close(wait)
	})); err != nil {
		return fmt.Errorf("%s watch daemon failed: %w", fmt.Sprintf("%s:%s", sys.getConfig().FetchName(), sys.getProcessManager().getHost()), err)
	}
	sys.shutdownLog(log.String("stage", "stopping"), log.String("info", "watching guard stop"))

	// 优雅关闭根 Actor
	sys.daemon.PoisonKill(sys.daemon.Ref(), "actor system shutdown")

	<-wait
	sys.shutdownLog(log.String("stage", "shutdown"), log.String("info", "completed"))
	return nil
}

// ShutdownP 关闭 Actor 系统，并在发生异常时 panic
func (sys *actorSystem) ShutdownP() ActorSystem {
	if err := sys.Shutdown(); err != nil {
		panic(err)
	}
	return sys
}
