package vivid

type ActorSystemConfigurator interface {
	Configure(system *ActorSystemConfig)
}

type ActorSystemConfiguratorFN func(system *ActorSystemConfig)

func (fn ActorSystemConfiguratorFN) Configure(system *ActorSystemConfig) {
	fn(system)
}
