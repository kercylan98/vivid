package vivid

type ActorSystemConfigurator interface {
	Configure(config *ActorSystemConfig)
}

type ActorSystemConfiguratorFN func(config *ActorSystemConfig)

func (fn ActorSystemConfiguratorFN) Configure(config *ActorSystemConfig) {
	fn(config)
}
