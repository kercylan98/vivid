package manager

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"github.com/kercylan98/vivid/src/process"
	"github.com/puzpuzpuz/xsync/v3"
)

var (
	_builder vivid.ManagerBuilder = &builder{}
)

func Builder() vivid.ManagerBuilder {
	return _builder
}

type builder struct{}

func (i *builder) OptionsOf(options vivid.ManagerOptions) vivid.Manager {
	mgr := i.Build().(*manager)
	mgr.options = options.(vivid.ManagerOptionsFetcher)
	return mgr
}

func (i *builder) ConfiguratorOf(configurator ...vivid.ManagerConfigurator) vivid.Manager {
	opts := Options()
	for _, c := range configurator {
		c.Configure(opts)
	}

	return i.OptionsOf(opts)
}

func (i *builder) Build() vivid.Manager {
	mgr := &manager{
		host:      "localhost",
		processes: xsync.NewMapOf[src.Path, vivid.Process](),
	}
	mgr.root = process.Builder().HostOf(mgr.host)
	return mgr
}
