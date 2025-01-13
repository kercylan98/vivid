package server

import (
	"github.com/kercylan98/vivid/core"
)

var _builder core.ServerBuilder = &builder{}

func Builder() core.ServerBuilder {
	return _builder
}

type builder struct{}

func (b *builder) Build() core.Server {
	return &server{
		options: Options().(core.ServerOptionsFetcher),
	}
}

func (b *builder) OptionsOf(options core.ServerOptions) core.Server {
	return &server{
		options: options.(core.ServerOptionsFetcher),
	}
}

func (b *builder) ConfiguratorOf(configurator ...core.ServerConfigurator) core.Server {
	opts := Options()
	for _, c := range configurator {
		c.Configure(opts)
	}
	return &server{
		options: opts.(core.ServerOptionsFetcher),
	}
}
