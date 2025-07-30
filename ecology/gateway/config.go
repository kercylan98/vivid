package gateway

import (
	"github.com/kercylan98/vivid/pkg/configurator"
	"github.com/kercylan98/vivid/pkg/serializer"
)

type (
	Configurator   = configurator.Configurator[*Configuration]
	ConfiguratorFN = configurator.FN[*Configuration]
	Option         = configurator.Option[*Configuration]
)

func NewConfiguration() *Configuration {
	return &Configuration{}
}

type (
	SessionMessageDecoder = serializer.Serializer
)

type Configuration struct {
	AssumeControllers []AssumeController
	CodecProvider     CodecProvider
}

func (c *Configuration) WithAssumeControllers(controllers ...AssumeController) *Configuration {
	c.AssumeControllers = append(c.AssumeControllers, controllers...)
	return c
}

func WithAssumeControllers(controllers ...AssumeController) ConfiguratorFN {
	return func(c *Configuration) {
		c.WithAssumeControllers(controllers...)
	}
}

func (c *Configuration) WithCodecProvider(provider CodecProvider) *Configuration {
	c.CodecProvider = provider
	return c
}

func WithCodecProvider(provider CodecProvider) ConfiguratorFN {
	return func(c *Configuration) {
		c.WithCodecProvider(provider)
	}
}
