package vivid

type ActorConfigurator interface {
	Configure(config *ActorConfig)
}

type ActorConfiguratorFn func(config *ActorConfig)

func (f ActorConfiguratorFn) Configure(config *ActorConfig) {
	f(config)
}
