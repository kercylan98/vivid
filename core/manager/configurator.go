package manager

import "github.com/kercylan98/vivid/core"

var _ core.ManagerConfigurator = Configurator(nil)

type Configurator func(options core.ManagerOptions)

func (c Configurator) Configure(options core.ManagerOptions) {
	c(options)
}
