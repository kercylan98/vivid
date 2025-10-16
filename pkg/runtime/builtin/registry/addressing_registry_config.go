package registry

import (
	"errors"

	"github.com/kercylan98/vivid/pkg/configurator"
	"github.com/kercylan98/vivid/pkg/provider"
	"github.com/kercylan98/vivid/pkg/runtime"
	"github.com/kercylan98/vivid/pkg/serializer"
)

func NewAddressingRegistryConfiguration() *AddressingRegistryConfiguration {
	return &AddressingRegistryConfiguration{}
}

type (
	AddressingRegistryConfigurator   = configurator.Configurator[*AddressingRegistryConfiguration]
	AddressingRegistryConfiguratorFN = configurator.FN[*AddressingRegistryConfiguration]
	AddressingRegistryOption         = configurator.Option[*AddressingRegistryConfiguration]
	AddressingRegistryConfiguration  struct {
		Serializer               serializer.NameSerializer                                              // 用于对自定义用户级消息进行序列化的序列化器
		BrokerSerializerProvider provider.Param1Provider[*runtime.ProcessID, serializer.NameSerializer] // Broker 序列化器提供器，如果未提供，则使用 Serializer 作为默认序列化器
		ServerConfig             *AddressingRegistryServerConfiguration                                 // 可用于跨进程通信的服务器配置，未配置时将仅为内存通讯
	}
)

func (c *AddressingRegistryConfiguration) validate() error {
	if c.Serializer == nil {
		return errors.New("serializer is required, please use WithAddressingRegistrySerializer to set the serializer")
	}
	if c.ServerConfig == nil {
		if c.ServerConfig.Server == nil {
			return errors.New("server is required, please use WithAddressingRegistryServer to set the server")
		}
		if c.ServerConfig.Transport == nil {
			return errors.New("transport is required, please use WithAddressingRegistryTransport to set the transport")
		}
		if c.ServerConfig.ConnectionPoolConfig == nil {
			return errors.New("connection pool config is required, please use WithAddressingRegistryConnectionPoolConfig to set the connection pool config")
		}
	}

	return nil
}

func (c *AddressingRegistryConfiguration) WithSerializer(serializer serializer.NameSerializer) *AddressingRegistryConfiguration {
	c.Serializer = serializer
	return c
}

func WithAddressingRegistrySerializer(serializer serializer.NameSerializer) AddressingRegistryOption {
	return func(c *AddressingRegistryConfiguration) {
		c.WithSerializer(serializer)
	}
}

func (c *AddressingRegistryConfiguration) WithBrokerSerializerProvider(provider provider.Param1Provider[*runtime.ProcessID, serializer.NameSerializer]) *AddressingRegistryConfiguration {
	c.BrokerSerializerProvider = provider
	return c
}

func WithAddressingRegistryBrokerSerializerProvider(provider provider.Param1Provider[*runtime.ProcessID, serializer.NameSerializer]) AddressingRegistryOption {
	return func(c *AddressingRegistryConfiguration) {
		c.WithBrokerSerializerProvider(provider)
	}
}

func (c *AddressingRegistryConfiguration) WithServerFromConfig(config *AddressingRegistryServerConfiguration) *AddressingRegistryConfiguration {
	c.ServerConfig = config
	return c
}

func WithAddressingRegistryServerFromConfig(server *AddressingRegistryServerConfiguration) AddressingRegistryOption {
	return func(c *AddressingRegistryConfiguration) {
		c.WithServerFromConfig(server)
	}
}

func (c *AddressingRegistryConfiguration) WithServerFromOptions(options ...AddressingRegistryServerOption) *AddressingRegistryConfiguration {
	config := NewAddressingRegistryServerConfiguration()
	for _, option := range options {
		option(config)
	}
	c.ServerConfig = config
	return c
}

func WithAddressingRegistryServerFromOptions(options ...AddressingRegistryServerOption) AddressingRegistryOption {
	return func(c *AddressingRegistryConfiguration) {
		c.WithServerFromOptions(options...)
	}
}

func (c *AddressingRegistryConfiguration) WithServerFromConfigurators(configurators ...AddressingRegistryServerConfigurator) *AddressingRegistryConfiguration {
	config := NewAddressingRegistryServerConfiguration()
	for _, configurator := range configurators {
		configurator.Configure(config)
	}
	c.ServerConfig = config
	return c
}

func WithAddressingRegistryServerFromConfigurators(configurators ...AddressingRegistryServerConfigurator) AddressingRegistryOption {
	return func(c *AddressingRegistryConfiguration) {
		c.WithServerFromConfigurators(configurators...)
	}
}
