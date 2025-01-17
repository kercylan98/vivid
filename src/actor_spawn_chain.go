package vivid

var (
	_ ActorSpawnChain = (*actorSpawnChain)(nil) // 确保 actorSpawnChain 实现了 ActorSpawnChain 接口
)

// ActorSpawnChain 是 Actor 生成链，用于生成 Actor
type ActorSpawnChain interface {
	// SetConfig 设置 ActorConfiguration
	SetConfig(config ActorConfiguration) ActorSpawnChain

	// SetConfigurator 设置 ActorConfigurator
	SetConfigurator(configurator ActorConfigurator) ActorSpawnChain

	// SetFnConfigurator 设置 ActorConfiguratorFn
	SetFnConfigurator(configurator ActorConfiguratorFn) ActorSpawnChain

	// AddNextConfigurator 添加 ActorConfigurator
	AddNextConfigurator(configurator ActorConfigurator) ActorSpawnChain

	// AddNextFnConfigurator 添加 ActorConfiguratorFn
	AddNextFnConfigurator(configurator ActorConfiguratorFn) ActorSpawnChain

	// ActorOf 生成 Actor
	ActorOf() ActorRef
}

func newActorSpawnChain(parent ActorContext, provider ActorProvider) ActorSpawnChain {
	return &actorSpawnChain{
		parent:   parent,
		provider: provider,
	}
}

type actorSpawnChain struct {
	parent        ActorContext
	provider      ActorProvider
	config        ActorConfiguration
	configurators []ActorConfigurator
}

func (a *actorSpawnChain) SetConfig(config ActorConfiguration) ActorSpawnChain {
	a.config = config
	return a
}

func (a *actorSpawnChain) SetConfigurator(configurator ActorConfigurator) ActorSpawnChain {
	a.configurators = []ActorConfigurator{configurator}
	return a
}

func (a *actorSpawnChain) SetFnConfigurator(configurator ActorConfiguratorFn) ActorSpawnChain {
	return a.SetConfigurator(configurator)
}

func (a *actorSpawnChain) AddNextConfigurator(configurator ActorConfigurator) ActorSpawnChain {
	a.configurators = append(a.configurators, configurator)
	return a
}

func (a *actorSpawnChain) AddNextFnConfigurator(configurator ActorConfiguratorFn) ActorSpawnChain {
	return a.AddNextConfigurator(configurator)
}

func (a *actorSpawnChain) ActorOf() ActorRef {
	if a.config == nil {
		a.config = NewActorConfig(a.parent)
	}
	for _, configurator := range a.configurators {
		configurator.Configure(a.config)
	}
	return a.parent.ActorOfConfig(a.provider, a.config)
}
