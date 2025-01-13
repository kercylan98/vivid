package server

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"github.com/kercylan98/vivid/src/resource"
)

var (
	_ vivid.ServerLauncher = (*addrLauncher)(nil)
)

func AddrLauncherBuilder(addr resource.Addr, server vivid.Server) vivid.ServerLauncherBuilder {
	return &addrLauncherBuilder{
		addr:   addr,
		server: server,
	}
}

type addrLauncherBuilder struct {
	addr   resource.Addr
	server vivid.Server
}

func (a *addrLauncherBuilder) Build() vivid.ServerLauncher {
	return &addrLauncher{
		addr:   a.addr,
		server: a.server,
	}
}

type addrLauncher struct {
	addr   resource.Addr
	server vivid.Server
}

func (a *addrLauncher) GetServer() vivid.Server {
	return a.server
}

func (a *addrLauncher) Launch() error {

}
