package manager

import (
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
	"github.com/kercylan98/vivid/.discord/src/process"
	"github.com/puzpuzpuz/xsync/v3"
)

var (
	_builder vivid2.ManagerBuilder = &builder{}
)

func Builder() vivid2.ManagerBuilder {
	return _builder
}

type builder struct{}

func (i *builder) OptionsOf(options vivid2.ManagerOptions) vivid2.Manager {
	mgr := i.Build().(*manager)
	mgr.options = options.(vivid2.ManagerOptionsFetcher)
	return mgr
}

func (i *builder) ConfiguratorOf(configurator ...vivid2.ManagerConfigurator) vivid2.Manager {
	opts := Options()
	for _, c := range configurator {
		c.Configure(opts)
	}

	return i.OptionsOf(opts)
}

func (i *builder) Build() vivid2.Manager {
	mgr := &manager{
		host:      "localhost",
		processes: xsync.NewMapOf[src.Path, vivid2.Process](),
	}
	mgr.root = process.Builder().HostOf(mgr.host)
	return mgr
}
