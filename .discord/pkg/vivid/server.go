package vivid

import (
	"net"
)

type ServerBuilder interface {
	Build() Server

	OptionsOf(options ServerOptions) Server

	ConfiguratorOf(configurator ...ServerConfigurator) Server
}

type Server interface {
	Serve(listener net.Listener) error

	GetConnChannel() <-chan Conn
}

type ServerOptions interface {
	WithConnChannelSize(channelSize int) ServerOptions
}

type ServerOptionsFetcher interface {
	GetConnChannelSize() int
}

type ServerConfigurator interface {
	Configure(options ServerOptions)
}

type FnServerConfigurator func(options ServerOptions)

func (c FnServerConfigurator) Configure(options ServerOptions) {
	c(options)
}

type ServerLauncherBuilder interface {
	Build() ServerLauncher
}

type ServerLauncher interface {
	GetServer() Server

	Launch() error
}
