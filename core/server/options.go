package server

import (
	"github.com/kercylan98/vivid/core"
	"github.com/kercylan98/vivid/core/envelope"
	"github.com/kercylan98/vivid/core/server/decoder"
)

var (
	_ core.ServerOptions        = (*options)(nil)
	_ core.ServerOptionsFetcher = (*options)(nil)
)

func Options() core.ServerOptions {
	return &options{
		decoderBuilder: decoder.Builder(),
		channelSize:    1024,
		envelopeProvider: core.FnEnvelopeProvider(func() core.Envelope {
			return envelope.Builder().EmptyOf()
		}),
	}
}

type options struct {
	decoderBuilder   core.DecoderBuilder
	envelopeProvider core.EnvelopeProvider
	channelSize      int
}

func (o *options) WithDecoderBuilder(decoderBuilder core.DecoderBuilder) core.ServerOptions {
	o.decoderBuilder = decoderBuilder
	return o
}

func (o *options) WithEnvelopeProvider(provider core.EnvelopeProvider) core.ServerOptions {
	o.envelopeProvider = provider
	return o
}

func (o *options) WithChannelSize(channelSize int) core.ServerOptions {
	o.channelSize = channelSize
	return o
}

func (o *options) GetDecoderBuilder() core.DecoderBuilder {
	return o.decoderBuilder
}

func (o *options) GetEnvelopeProvider() core.EnvelopeProvider {
	return o.envelopeProvider
}

func (o *options) GetChannelSize() int {
	return o.channelSize
}
