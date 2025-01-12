package server

import "github.com/kercylan98/vivid/core"

var _builder core.ServerBuilder = &builder{}

func Builder() core.ServerBuilder {
	return _builder
}

type builder struct{}

func (b *builder) Build(decoderBuilder core.DecoderBuilder, provider core.EnvelopeProvider, channelSize int) core.Server {
	return &server{
		envelopeProvider: provider,
		decoderBuilder:   decoderBuilder,
		envelopeChannel:  make(chan core.Envelope, channelSize),
	}
}
