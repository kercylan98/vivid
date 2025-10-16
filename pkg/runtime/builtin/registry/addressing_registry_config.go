package registry

import (
	"errors"
	"io"

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
		Address                  string                                                                 // 地址
		Serializer               serializer.NameSerializer                                              // 在 Broker 未指定序列化器时使用的默认序列化器
		BrokerSerializerProvider provider.Param1Provider[*runtime.ProcessID, serializer.NameSerializer] // 该提供器被用于在 Broker 创建前获取其所需的序列化器，如果未提供，则使用 Serializer 作为默认序列化器
		BrokerWriterProvider     provider.Param1Provider[*runtime.ProcessID, io.Writer]                 // 该提供器被用于在 Broker 创建前获取其所需的写入器，Broker 将会将消息写入到该写入器中
	}
)

func (c *AddressingRegistryConfiguration) validate() error {
	if c.Serializer == nil {
		return errors.New("serializer is required, please use WithAddressingRegistrySerializer to set the serializer")
	}
	if c.BrokerWriterProvider == nil {
		return errors.New("broker writer provider is required, please use WithAddressingRegistryBrokerWriterProvider to set the broker writer provider")
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

func (c *AddressingRegistryConfiguration) WithAddress(address string) *AddressingRegistryConfiguration {
	c.Address = address
	return c
}

func WithAddressingRegistryAddress(address string) AddressingRegistryOption {
	return func(c *AddressingRegistryConfiguration) {
		c.WithAddress(address)
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

func (c *AddressingRegistryConfiguration) WithBrokerWriterProvider(provider provider.Param1Provider[*runtime.ProcessID, io.Writer]) *AddressingRegistryConfiguration {
	c.BrokerWriterProvider = provider
	return c
}

func WithAddressingRegistryBrokerWriterProvider(provider provider.Param1Provider[*runtime.ProcessID, io.Writer]) AddressingRegistryOption {
	return func(c *AddressingRegistryConfiguration) {
		c.WithBrokerWriterProvider(provider)
	}
}
