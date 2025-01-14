package manager

import (
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
)

var (
	_ vivid2.ManagerOptions        = (*options)(nil)
	_ vivid2.ManagerOptionsFetcher = (*options)(nil)
)

func Options() vivid2.ManagerOptions {
	return &options{}
}

type options struct {
	serverLauncher vivid2.ServerLauncher
}

func (o *options) FetchServer() vivid2.ServerLauncher {
	return o.serverLauncher
}

func (o *options) WithServer(launcher vivid2.ServerLauncher) vivid2.ManagerOptions {
	o.serverLauncher = launcher
	return o
}
