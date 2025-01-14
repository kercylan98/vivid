package server

import (
	"github.com/kercylan98/vivid/.discord/pkg/vivid"
)

var _builder vivid.ServerBuilder = &builder{}

func Builder() vivid.ServerBuilder {
	return _builder
}

type builder struct{}

func (b *builder) Build() vivid.Server {
	return &server{
		options: Options().(vivid.ServerOptionsFetcher),
	}
}

func (b *builder) OptionsOf(options vivid.ServerOptions) vivid.Server {
	return &server{
		options: options.(vivid.ServerOptionsFetcher),
	}
}

func (b *builder) ConfiguratorOf(configurator ...vivid.ServerConfigurator) vivid.Server {
	opts := Options()
	for _, c := range configurator {
		c.Configure(opts)
	}
	return &server{
		options: opts.(vivid.ServerOptionsFetcher),
	}
}
