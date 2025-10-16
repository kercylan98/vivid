package registry

import "github.com/kercylan98/vivid/pkg/configurator"

func NewAddressingRegistryServerConfiguration() *AddressingRegistryServerConfiguration {
	return &AddressingRegistryServerConfiguration{}
}

type (
	AddressingRegistryServerOption         = configurator.Option[*AddressingRegistryServerConfiguration]
	AddressingRegistryServerConfigurator   = configurator.Configurator[*AddressingRegistryServerConfiguration]
	AddressingRegistryServerConfiguratorFN = configurator.FN[*AddressingRegistryServerConfiguration]
	AddressingRegistryServerConfiguration  struct {
		AdvertiseAddress     string                       // 广告地址，用于向其他节点公布自己的地址
		Server               Server                       // 服务器
		Transport            Transport                    // 传输层
		ConnectionPoolConfig *ConnectionPoolConfiguration // 连接池配置
	}
)

func (c *AddressingRegistryServerConfiguration) WithAdvertiseAddress(address string) *AddressingRegistryServerConfiguration {
	c.AdvertiseAddress = address
	return c
}

func WithAddressingRegistryServerAdvertiseAddress(address string) AddressingRegistryServerOption {
	return func(c *AddressingRegistryServerConfiguration) {
		c.WithAdvertiseAddress(address)
	}
}

func (c *AddressingRegistryServerConfiguration) WithServer(server Server) *AddressingRegistryServerConfiguration {
	c.Server = server
	return c
}

func WithAddressingRegistryServerServer(server Server) AddressingRegistryServerOption {
	return func(c *AddressingRegistryServerConfiguration) {
		c.WithServer(server)
	}
}

func (c *AddressingRegistryServerConfiguration) WithTransport(transport Transport) *AddressingRegistryServerConfiguration {
	c.Transport = transport
	return c
}

func WithAddressingRegistryServerTransport(transport Transport) AddressingRegistryServerOption {
	return func(c *AddressingRegistryServerConfiguration) {
		c.WithTransport(transport)
	}
}

func (c *AddressingRegistryServerConfiguration) WithConnectionPoolFromConfig(config *ConnectionPoolConfiguration) *AddressingRegistryServerConfiguration {
	c.ConnectionPoolConfig = config
	return c
}

func WithAddressingRegistryServerConnectionPoolFromConfig(config *ConnectionPoolConfiguration) AddressingRegistryServerOption {
	return func(c *AddressingRegistryServerConfiguration) {
		c.WithConnectionPoolFromConfig(config)
	}
}

func (c *AddressingRegistryServerConfiguration) WithConnectionPoolFromOptions(options ...ConnectionPoolOption) *AddressingRegistryServerConfiguration {
	config := NewConnectionPoolConfiguration()
	for _, option := range options {
		option(config)
	}
	c.ConnectionPoolConfig = config
	return c
}

func WithAddressingRegistryServerConnectionPoolFromOptions(options ...ConnectionPoolOption) AddressingRegistryServerOption {
	return func(c *AddressingRegistryServerConfiguration) {
		c.WithConnectionPoolFromOptions(options...)
	}
}

func (c *AddressingRegistryServerConfiguration) WithConnectionPoolFromConfigurators(configurators ...ConnectionPoolConfigurator) *AddressingRegistryServerConfiguration {
	config := NewConnectionPoolConfiguration()
	for _, configurator := range configurators {
		configurator.Configure(config)
	}
	c.ConnectionPoolConfig = config
	return c
}

func WithAddressingRegistryServerConnectionPoolFromConfigurators(configurators ...ConnectionPoolConfigurator) AddressingRegistryServerOption {
	return func(c *AddressingRegistryServerConfiguration) {
		c.WithConnectionPoolFromConfigurators(configurators...)
	}
}
