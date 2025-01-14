package server

import (
	"github.com/kercylan98/vivid/.discord/pkg/vivid"
	"net"
)

var (
	_ vivid.ServerLauncher = (*addrLauncher)(nil)
)

func ListenerLauncherBuilder(listener net.Listener, server vivid.Server) vivid.ServerLauncherBuilder {
	return &listenerLauncherBuilder{
		listener: listener,
		server:   server,
	}
}

type listenerLauncherBuilder struct {
	listener net.Listener
	server   vivid.Server
}

func (a *listenerLauncherBuilder) Build() vivid.ServerLauncher {
	return &listenerLauncher{
		listener: a.listener,
		server:   a.server,
	}
}

type listenerLauncher struct {
	listener net.Listener
	server   vivid.Server
}

func (a *listenerLauncher) GetServer() vivid.Server {
	return a.server
}

func (a *listenerLauncher) Launch() error {

}
