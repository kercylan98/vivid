package system

import (
	"context"
	"time"

	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/wasteland/src/wasteland"
)

var _ actor.System = (*System)(nil)

func New(config Config) *System {
	if config.LoggerProvider == nil {
		config.LoggerProvider = log.ProviderFn(log.GetBuilder().Production)
	}

	if config.TimingWheelTick <= 0 {
		config.TimingWheelTick = time.Millisecond * 50
	}
	if config.TimingWheelSize <= 0 {
		config.TimingWheelSize = 20
	}

	system := &System{
		config: &config,
		timingWheel: timing.New(timing.ConfiguratorFN(func(timingConfig timing.Configuration) {
			timingConfig.
				WithSize(config.TimingWheelSize).
				WithTick(config.TimingWheelTick).
				WithExecutor(timing.ExecutorFN(func(task func()) {
					task() // 不再 recover，以使监管策略生效
				}))
		})),
	}
	system.ctx, system.cancel = context.WithCancel(context.Background())

	return system
}

type System struct {
	config           *Config                   // 系统配置
	locator          wasteland.ResourceLocator // ActorSystem 的资源定位符
	guide            actor.Context             // 顶级守护 Actor
	registry         wasteland.ProcessRegistry // 进程注册表
	ctx              context.Context           // 系统上下文
	cancel           context.CancelFunc        // 系统上下文取消函数
	timingWheel      timing.Wheel              // 系统时间轮
	globalMonitoring interface{}               // 全局监控实例
}

func (s *System) Register(ctx actor.Context) {
	if err := s.registry.Register(ctx.ProcessContext()); err != nil {
		panic(err)
	}
}

func (s *System) Unregister(operator, target actor.Ref) {
	s.registry.Unregister(operator, target)
}

func (s *System) Registry() wasteland.ProcessRegistry {
	return s.registry
}

func (s *System) Find(target actor.Ref) wasteland.ProcessHandler {
	// System 会设置守护进程，所以这里可忽略错误
	process, _ := s.registry.Get(target)
	return process.(wasteland.ProcessHandler)
}

func (s *System) LoggerProvider() log.Provider {
	return s.config.LoggerProvider
}

func (s *System) ResourceLocator() wasteland.ResourceLocator {
	return s.locator
}

func (s *System) Run() error {
	s.locator = wasteland.NewResourceLocator(s.config.Address, "/")

	// 为Guard Actor创建配置，包含全局监控
	guardConfig := actor.Config{
		Supervisor: actx.GetDefaultSupervisor(s.config.GuardDefaultRestartLimit),
	}

	// 如果有全局监控，将其设置到Guard Actor配置中
	if s.globalMonitoring != nil {
		if adapter, ok := s.globalMonitoring.(actor.Metrics); ok {
			guardConfig.Monitoring = adapter
		}
	}

	s.guide = (*actx.Generate)(nil).GenerateActorContext(s, nil, GuardProvider(s.cancel), guardConfig)
	s.registry = wasteland.NewProcessRegistry(wasteland.ProcessRegistryConfig{
		Locator:           s.ResourceLocator(),
		Daemon:            s.guide.ProcessContext(),
		LoggerProvide:     s.config.LoggerProvider,
		CodecProvider:     s.config.CodecProvider,
		RPCMessageBuilder: s.config.RPCMessageBuilder,
	})
	s.Register(s.guide)
	s.guide.TransportContext().Tell(s.guide.MetadataContext().Ref(), actx.SystemMessage, actor.OnLaunchMessageInstance)
	return s.registry.Run()
}

func (s *System) Stop() error {
	// 向守护Actor发送终止消息
	s.guide.TransportContext().Tell(s.guide.MetadataContext().Ref(), actx.SystemMessage, &actor.OnKill{
		Reason:   "actor system stop",
		Operator: s.guide.MetadataContext().Ref(),
		Poison:   false,
	})

	// 根据配置决定是否使用超时机制
	if s.config.StopTimeout > 0 {
		// 使用配置的超时时间
		done := make(chan struct{}, 1)
		go func() {
			<-s.ctx.Done()
			done <- struct{}{}
		}()

		select {
		case <-done:
			// 正常停止
		case <-time.After(s.config.StopTimeout):
			// 超时强制取消
			s.cancel()
		}
	} else {
		// 无超时，等待系统正常停止
		<-s.ctx.Done()
	}

	// 停止进程注册表
	s.registry.Stop()

	return nil
}

func (s *System) PoisonStop() error {
	// 向守护Actor发送优雅终止消息
	s.guide.TransportContext().Tell(s.guide.MetadataContext().Ref(), actx.UserMessage, &actor.OnKill{
		Reason:   "actor system stop with poison",
		Operator: s.guide.MetadataContext().Ref(),
		Poison:   true,
	})

	// 根据配置决定是否使用超时机制
	if s.config.PoisonStopTimeout > 0 {
		// 使用配置的超时时间
		done := make(chan struct{}, 1)
		go func() {
			<-s.ctx.Done()
			done <- struct{}{}
		}()

		select {
		case <-done:
			// 正常停止
		case <-time.After(s.config.PoisonStopTimeout):
			// 超时强制取消
			s.cancel()
		}
	} else {
		// 无超时，等待系统正常停止
		<-s.ctx.Done()
	}

	// 停止进程注册表
	s.registry.Stop()

	return nil
}

func (s *System) Context() actor.Context {
	return s.guide
}

func (s *System) GetTimingWheel() timing.Wheel {
	return s.timingWheel
}

// SetGlobalMonitoring 设置全局监控实例
func (s *System) SetGlobalMonitoring(monitoring interface{}) {
	s.globalMonitoring = monitoring
}

// GetGlobalMonitoring 获取全局监控实例
func (s *System) GetGlobalMonitoring() interface{} {
	return s.globalMonitoring
}
