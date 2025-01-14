package conn

import (
	"bytes"
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
	"github.com/kercylan98/vivid/.discord/src/envelope"
)

var (
	_              vivid2.ConnReaderBuilder = (*readerBuilder)(nil)
	_readerBuilder vivid2.ConnReaderBuilder = &readerBuilder{}
)

func ReaderBuilder() vivid2.ConnReaderBuilder {
	return _readerBuilder
}

type readerBuilder struct {
}

func (b *readerBuilder) Build(conn vivid2.Conn, decoderBuilder vivid2.DecoderBuilder) vivid2.ConnReader {
	r := &reader{
		conn:   conn,
		buffer: new(bytes.Buffer),
	}
	r.decoder = decoderBuilder.Build(r.buffer, vivid2.EnvelopeProviderFn(func() vivid2.Envelope {
		return envelope.Builder().EmptyOf()
	}))
	return r
}

func (b *readerBuilder) OptionsOf(conn vivid2.Conn, decoderBuilder vivid2.DecoderBuilder, options vivid2.ConnReaderOptions) vivid2.ConnReader {
	r := b.Build(conn, decoderBuilder).(*reader)
	r.options = options.(vivid2.ConnReaderOptionsFetcher)
	return r
}

func (b *readerBuilder) ConfiguratorOf(conn vivid2.Conn, decoderBuilder vivid2.DecoderBuilder, configurator ...vivid2.ConnConfigurator) vivid2.ConnReader {
	r := b.Build(conn, decoderBuilder).(*reader)
	options := ReaderOptions()
	for _, c := range configurator {
		c.Configure(options)
	}
	r.options = options.(vivid2.ConnReaderOptionsFetcher)
	return r
}
