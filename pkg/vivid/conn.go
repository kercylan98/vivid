package vivid

type Conn interface {
	Read(b []byte) (n int, err error)
	Close() error
}

type ConnReaderBuilder interface {
	Build(conn Conn, decoderBuilder DecoderBuilder) ConnReader

	OptionsOf(conn Conn, decoderBuilder DecoderBuilder, options ConnReaderOptions) ConnReader

	ConfiguratorOf(conn Conn, decoderBuilder DecoderBuilder, configurator ...ConnConfigurator) ConnReader
}

type ConnReader interface {
	Read(chan<- Envelope)
}

type ConnReaderOptions interface {
}

type ConnReaderOptionsFetcher interface {
}

type ConnConfigurator interface {
	Configure(options ...ConnReaderOptions)
}

type FnConnConfigurator func(options ...ConnReaderOptions)

func (c FnConnConfigurator) Configure(options ...ConnReaderOptions) {
	c(options...)
}
