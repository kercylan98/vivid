package core

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

	GetEnvelopeChannel() <-chan Envelope
}

type ServerOptions interface {
	WithDecoderBuilder(decoderBuilder DecoderBuilder) ServerOptions
	WithEnvelopeProvider(provider EnvelopeProvider) ServerOptions
	WithChannelSize(channelSize int) ServerOptions
}

type ServerOptionsFetcher interface {
	GetDecoderBuilder() DecoderBuilder
	GetEnvelopeProvider() EnvelopeProvider
	GetChannelSize() int
}

type ServerConfigurator interface {
	Configure(options ServerOptions)
}

type FnServerConfigurator func(options ServerOptions)

func (c FnServerConfigurator) Configure(options ServerOptions) {
	c(options)
}
