package registry

import (
	"time"

	"github.com/kercylan98/vivid/pkg/configurator"
)

// NewConnectionPoolConfiguration 创建一个新的连接池配置。
func NewConnectionPoolConfiguration() *ConnectionPoolConfiguration {
	return &ConnectionPoolConfiguration{
		IdleTimeout:          time.Minute * 5,
		MaxConnsPerAddress:   1,
		SendQueueSize:        256,
		HandshakeTimeout:     time.Second * 10,
		ReconnectInterval:    time.Second * 5,
		MaxReconnectAttempts: 3,
	}
}

type (
	ConnectionPoolConfigurator   = configurator.Configurator[*ConnectionPoolConfiguration]
	ConnectionPoolConfiguratorFN = configurator.FN[*ConnectionPoolConfiguration]
	ConnectionPoolOption         = configurator.Option[*ConnectionPoolConfiguration]
	ConnectionPoolConfiguration  struct {
		Transport            Transport     // 传输层
		IdleTimeout          time.Duration // 空闲超时
		MaxConnsPerAddress   int           // 每地址最大连接数
		SendQueueSize        int           // 发送队列大小
		HandshakeTimeout     time.Duration // 握手超时
		ReconnectInterval    time.Duration // 重连间隔
		MaxReconnectAttempts int           // 最大重连次数
	}
)

func (c *ConnectionPoolConfiguration) WithTransport(transport Transport) *ConnectionPoolConfiguration {
	c.Transport = transport
	return c
}

func WithConnectionPoolTransport(transport Transport) ConnectionPoolOption {
	return func(c *ConnectionPoolConfiguration) {
		c.WithTransport(transport)
	}
}

func (c *ConnectionPoolConfiguration) WithIdleTimeout(timeout time.Duration) *ConnectionPoolConfiguration {
	c.IdleTimeout = timeout
	return c
}

func WithConnectionPoolIdleTimeout(timeout time.Duration) ConnectionPoolOption {
	return func(c *ConnectionPoolConfiguration) {
		c.WithIdleTimeout(timeout)
	}
}

func (c *ConnectionPoolConfiguration) WithMaxConnsPerAddress(max int) *ConnectionPoolConfiguration {
	c.MaxConnsPerAddress = max
	return c
}

func WithConnectionPoolMaxConnsPerAddress(max int) ConnectionPoolOption {
	return func(c *ConnectionPoolConfiguration) {
		c.WithMaxConnsPerAddress(max)
	}
}

func (c *ConnectionPoolConfiguration) WithSendQueueSize(size int) *ConnectionPoolConfiguration {
	c.SendQueueSize = size
	return c
}

func WithConnectionPoolSendQueueSize(size int) ConnectionPoolOption {
	return func(c *ConnectionPoolConfiguration) {
		c.WithSendQueueSize(size)
	}
}

func (c *ConnectionPoolConfiguration) WithHandshakeTimeout(timeout time.Duration) *ConnectionPoolConfiguration {
	c.HandshakeTimeout = timeout
	return c
}

func WithConnectionPoolHandshakeTimeout(timeout time.Duration) ConnectionPoolOption {
	return func(c *ConnectionPoolConfiguration) {
		c.WithHandshakeTimeout(timeout)
	}
}

func (c *ConnectionPoolConfiguration) WithReconnectInterval(interval time.Duration) *ConnectionPoolConfiguration {
	c.ReconnectInterval = interval
	return c
}

func WithConnectionPoolReconnectInterval(interval time.Duration) ConnectionPoolOption {
	return func(c *ConnectionPoolConfiguration) {
		c.WithReconnectInterval(interval)
	}
}

func (c *ConnectionPoolConfiguration) WithMaxReconnectAttempts(max int) *ConnectionPoolConfiguration {
	c.MaxReconnectAttempts = max
	return c
}

func WithConnectionPoolMaxReconnectAttempts(max int) ConnectionPoolOption {
	return func(c *ConnectionPoolConfiguration) {
		c.WithMaxReconnectAttempts(max)
	}
}
