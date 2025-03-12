package vivid

func newActorContextConfigurationProvider(config *ActorConfiguration) actorContextConfigurationProvider {
	return &actorContextConfigurationProviderImpl{
		config: config,
	}
}

type actorContextConfigurationProvider interface {
	getConfig() *ActorConfiguration
}

type actorContextConfigurationProviderImpl struct {
	config *ActorConfiguration
}

func (a *actorContextConfigurationProviderImpl) getConfig() *ActorConfiguration {
	return a.config
}
