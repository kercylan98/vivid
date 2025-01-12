package manager

import "github.com/kercylan98/vivid/core"

var (
	_ core.ManagerOptions        = (*options)(nil)
	_ core.ManagerOptionsFetcher = (*options)(nil)
)

func Options() core.ManagerOptions {
	return &options{}
}

type options struct {
	server core.Server
}

func (opts *options) WithServer(server core.Server) core.ManagerOptions {
	opts.server = server
	return opts
}

func (opts *options) FetchServer() core.Server {
	return opts.server
}
