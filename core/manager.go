package core

type ManagerBuilder interface {
	Build() Manager

	OptionsOf(options ManagerOptions) Manager

	ConfiguratorOf(configurator ...ManagerConfigurator) Manager
}

type Manager interface {
	Run() error

	GetHost() Host

	RegisterProcess(process Process) (id ID, exist bool)

	UnregisterProcess(operator, id ID)

	GetProcess(id ID) Process
}

type ManagerOptions interface {
	WithServer(server Server) ManagerOptions
}

type ManagerOptionsFetcher interface {
	FetchServer() Server
}

type ManagerConfigurator interface {
	Configure(options ManagerOptions)
}

type FnManagerConfigurator func(options ManagerOptions)

func (c FnManagerConfigurator) Configure(options ManagerOptions) {
	c(options)
}
