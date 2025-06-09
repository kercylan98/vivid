package vivid

func StandardSupervisorFromConfig(config *StandardSupervisorConfiguration) Supervisor {
	return &standardSupervisor{
		config: *config,
	}
}

func StandardSupervisorWithConfigurators(configurators ...StandardSupervisorConfigurator) Supervisor {
	config := NewStandardSupervisorConfiguration()
	for _, c := range configurators {
		c.Configure(config)
	}
	return StandardSupervisorFromConfig(config)
}

func StandardSupervisorWithOptions(options ...StandardSupervisorOption) Supervisor {
	config := NewStandardSupervisorConfiguration(options...)
	return StandardSupervisorFromConfig(config)
}

type standardSupervisor struct {
	config StandardSupervisorConfiguration
}

func (s *standardSupervisor) Strategy(fatal *Fatal) SupervisorDirective {
	var directive = DirectiveRestart
	if s.config.DirectiveProvider != nil {
		directive = s.config.DirectiveProvider.Provide(fatal)
	}
	return directive
}
