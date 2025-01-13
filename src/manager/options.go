package manager

import (
	"github.com/kercylan98/vivid/pkg/vivid"
)

var (
	_ vivid.ManagerOptions        = (*options)(nil)
	_ vivid.ManagerOptionsFetcher = (*options)(nil)
)

func Options() vivid.ManagerOptions {
	return &options{}
}

type options struct {
	serverLauncher vivid.ServerLauncher
}

func (o *options) FetchServer() vivid.ServerLauncher {
	return o.serverLauncher
}

func (o *options) WithServer(launcher vivid.ServerLauncher) vivid.ManagerOptions {
	o.serverLauncher = launcher
	return o
}
