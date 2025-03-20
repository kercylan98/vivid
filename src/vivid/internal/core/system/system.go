package system

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/actx"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/messages"
	"github.com/kercylan98/wasteland/src/wasteland"
)

var _ actor.System = (*System)(nil)

func New(config Config) *System {
	if config.LoggerProvider == nil {
		config.LoggerProvider = log.ProviderFn(log.GetDefault)
	}

	system := &System{
		config: &config,
	}

	return system
}

type System struct {
	config   *Config
	meta     wasteland.Meta
	guide    actor.Context
	registry wasteland.ProcessRegistry
}

func (s *System) Register(ctx actor.Context) {
	if err := s.registry.Register(ctx.ProcessContext()); err != nil {
		panic(err)
	}
}

func (s *System) Find(target actor.Ref) wasteland.ProcessHandler {
	// System 会设置守护进程，所以这里可忽略错误
	process, _ := s.registry.Get(target)
	return process.(wasteland.ProcessHandler)
}

func (s *System) LoggerProvider() log.Provider {
	return s.config.LoggerProvider
}

func (s *System) Meta() wasteland.Meta {
	return s.meta
}

func (s *System) Run() error {
	s.meta = wasteland.NewMeta(s.config.Address)
	s.guide = (*actx.Generate)(nil).GenerateActorContext(s, nil, GuardProvider(), actor.Config{})
	s.registry = wasteland.NewProcessRegistry(wasteland.ProcessRegistryConfig{
		Meta:          s.Meta(),
		Daemon:        s.guide.ProcessContext(),
		LoggerProvide: s.config.LoggerProvider,
	})
	s.guide.TransportContext().Tell(s.guide.MetadataContext().Ref(), actx.SystemMessage, messages.OnLaunchMessageInstance)
	return s.registry.Run()
}

func (s *System) Shutdown() error {
	s.registry.Stop()
	return nil
}

func (s *System) Context() actor.Context {
	return s.guide
}
