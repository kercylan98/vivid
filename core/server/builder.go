package server

import (
	"github.com/kercylan98/vivid/core"
	"github.com/kercylan98/vivid/core/envelope"
	"github.com/kercylan98/vivid/core/server/decoder"
)

var _builder core.ServerBuilder = &builder{}

func Builder() core.ServerBuilder {
	return _builder
}

type builder struct{}

func (b *builder) DefaultOf() core.Server {
	return b.Build(
		decoder.Builder(),
		core.FnEnvelopeProvider(func() core.Envelope {
			return envelope.Builder().EmptyOf()
		}),
		1024,
	)
}

func (b *builder) Build(decoderBuilder core.DecoderBuilder, provider core.EnvelopeProvider, channelSize int) core.Server {
	return &server{
		envelopeProvider: provider,
		decoderBuilder:   decoderBuilder,
		envelopeChannel:  make(chan core.Envelope, channelSize),
	}
}
