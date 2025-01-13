package manager

import (
	"github.com/kercylan98/vivid/pkg/vivid"
)

var _ vivid.ManagerConfigurator = Configurator(nil)

type Configurator func(options vivid.ManagerOptions)

func (c Configurator) Configure(options vivid.ManagerOptions) {
	c(options)
}
