package vivid

type ActorConfigurator interface {
	Configure(config *ActorConfig)
}

type ActorConfiguratorFN func(config *ActorConfig)

func (f ActorConfiguratorFN) Configure(config *ActorConfig) {
	f(config)
}
