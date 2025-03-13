package vividtemp

func newActorContextConfigurationProvider(ctx ActorContext, config *ActorConfiguration) actorContextConfigurationProvider {
	return &actorContextConfigurationProviderImpl{
		ctx:    ctx,
		config: config,
	}
}

type actorContextConfigurationProvider interface {
	getConfig() *ActorConfiguration
}

type actorContextConfigurationProviderImpl struct {
	ctx    ActorContext
	config *ActorConfiguration
}

func (a *actorContextConfigurationProviderImpl) getConfig() *ActorConfiguration {
	return a.config
}
