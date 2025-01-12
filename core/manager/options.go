package manager

import "github.com/kercylan98/vivid/core"

var (
	_ core.ManagerOptions        = (*Options)(nil)
	_ core.ManagerOptionsFetcher = (*Options)(nil)
)

type Options struct {
	server core.Server
}

func (opts *Options) WithServer(server core.Server) core.ManagerOptions {
	opts.server = server
	return opts
}

func (opts *Options) FetchServer() core.Server {
	return opts.server
}
