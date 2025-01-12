package manager

import (
	"github.com/kercylan98/vivid/core"
	"github.com/kercylan98/vivid/core/process"
	"github.com/puzpuzpuz/xsync/v3"
)

var (
	_builder core.ManagerBuilder = &builder{}
)

func Builder() core.ManagerBuilder {
	return _builder
}

type builder struct{}

func (i *builder) OptionsOf(options ...core.ManagerOptions) core.Manager {
	//TODO implement me
	panic("implement me")
}

func (i *builder) ConfiguratorOf(configurator ...core.ManagerConfigurator) core.Manager {
	//TODO implement me
	panic("implement me")
}

func (i *builder) Build() core.Manager {
	mgr := &manager{
		host:      "localhost",
		processes: xsync.NewMapOf[core.Path, core.Process](),
	}
	mgr.root = process.Builder().HostOf(mgr.host)
	return mgr
}
