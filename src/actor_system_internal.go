package vivid

import "github.com/kercylan98/chrono/timing"

var (
	_ actorSystemInternal = (*actorSystemInternalImpl)(nil)
)

type actorSystemInternal interface {
	setConfig(config ActorSystemOptionsFetcher)

	getConfig() ActorSystemOptionsFetcher

	getProcessManager() processManager

	getTimingWheel() timing.Wheel
}

func newActorSystemInternal(system ActorSystem, config ActorSystemOptionsFetcher, processManager processManager) actorSystemInternal {
	return &actorSystemInternalImpl{
		ActorSystem:    system,
		config:         config,
		processManager: processManager,
		timingWheel: timing.New(timing.ConfiguratorFn(func(config timing.Configuration) {
			config.WithSize(50)
		})),
	}
}

type actorSystemInternalImpl struct {
	ActorSystem
	config         ActorSystemOptionsFetcher
	processManager processManager
	timingWheel    timing.Wheel
}

func (a *actorSystemInternalImpl) getConfig() ActorSystemOptionsFetcher {
	return a.config
}

func (a *actorSystemInternalImpl) setConfig(config ActorSystemOptionsFetcher) {
	a.config = config
}

func (a *actorSystemInternalImpl) getProcessManager() processManager {
	return a.processManager
}

func (a *actorSystemInternalImpl) getTimingWheel() timing.Wheel {
	return a.timingWheel
}
