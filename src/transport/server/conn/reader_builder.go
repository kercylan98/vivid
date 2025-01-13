package conn

import (
	"bytes"
	"github.com/kercylan98/vivid/pkg/vivid"
	"github.com/kercylan98/vivid/src/envelope"
)

var (
	_              vivid.ConnReaderBuilder = (*readerBuilder)(nil)
	_readerBuilder vivid.ConnReaderBuilder = &readerBuilder{}
)

func ReaderBuilder() vivid.ConnReaderBuilder {
	return _readerBuilder
}

type readerBuilder struct {
}

func (b *readerBuilder) Build(conn vivid.Conn, decoderBuilder vivid.DecoderBuilder) vivid.ConnReader {
	r := &reader{
		conn:   conn,
		buffer: new(bytes.Buffer),
	}
	r.decoder = decoderBuilder.Build(r.buffer, vivid.FnEnvelopeProvider(func() vivid.Envelope {
		return envelope.Builder().EmptyOf()
	}))
	return r
}

func (b *readerBuilder) OptionsOf(conn vivid.Conn, decoderBuilder vivid.DecoderBuilder, options vivid.ConnReaderOptions) vivid.ConnReader {
	r := b.Build(conn, decoderBuilder).(*reader)
	r.options = options.(vivid.ConnReaderOptionsFetcher)
	return r
}

func (b *readerBuilder) ConfiguratorOf(conn vivid.Conn, decoderBuilder vivid.DecoderBuilder, configurator ...vivid.ConnConfigurator) vivid.ConnReader {
	r := b.Build(conn, decoderBuilder).(*reader)
	options := ReaderOptions()
	for _, c := range configurator {
		c.Configure(options)
	}
	r.options = options.(vivid.ConnReaderOptionsFetcher)
	return r
}
