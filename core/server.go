package core

import (
	"net"
)

type ServerBuilder interface {
	Build(decoderBuilder DecoderBuilder, provider EnvelopeProvider, channelSize int) Server

	DefaultOf() Server
}

type Server interface {
	Serve(listener net.Listener) error

	GetEnvelopeChannel() <-chan Envelope
}
