package vivid

var (
	_ actorSystemInternal = (*actorSystemInternalImpl)(nil)
)

func newActorSystemInternal(system ActorSystem, config ActorSystemOptionsFetcher, processManager processManager) actorSystemInternal {
	return &actorSystemInternalImpl{
		ActorSystem:    system,
		config:         config,
		processManager: processManager,
	}
}

type actorSystemInternalImpl struct {
	ActorSystem
	config         ActorSystemOptionsFetcher
	processManager processManager
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
