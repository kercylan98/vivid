package vivid

import (
	"time"

	"github.com/kercylan98/vivid/pkg/provider"
	"github.com/kercylan98/vivid/pkg/serializer"
	"github.com/kercylan98/vivid/pkg/vivid/processor"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/pkg/configurator"
)

// NewActorSystemConfiguration 创建新的ActorSystem配置实例
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *ActorSystemConfiguration: 配置实例
func NewActorSystemConfiguration(options ...ActorSystemOption) *ActorSystemConfiguration {
	c := &ActorSystemConfiguration{
		Logger: log.GetBuilder().FromConfigurators(log.LoggerConfiguratorFn(func(config log.LoggerConfiguration) {
			config.
				WithLeveler(log.LevelInfo).
				WithEnableColor(true).
				WithErrTrackLevel(log.LevelError).
				WithTrackBeautify(true).
				WithMessageFormatter(func(message string) string {
					return message
				})
		})),
		FutureDefaultTimeout: time.Second,
		HTW:                  *DefaultHtwConfig(),
		HTWEnabled:           true, // 默认启用
	}
	for _, option := range options {
		option(c)
	}
	return c
}

func NewActorSystemNetworkConfiguration(options ...ActorSystemNetworkOption) *ActorSystemNetworkConfiguration {
	c := &ActorSystemNetworkConfiguration{
		Network:     "tcp",
		BindAddress: "127.0.0.1:0",
	}
	for _, option := range options {
		option(c)
	}
	return c
}

type (
	// ActorSystemConfigurator 配置器接口
	ActorSystemConfigurator = configurator.Configurator[*ActorSystemConfiguration]

	// ActorSystemConfiguratorFN 配置器函数类型
	ActorSystemConfiguratorFN = configurator.FN[*ActorSystemConfiguration]

	// ActorSystemOption 配置选项函数类型
	ActorSystemOption = configurator.Option[*ActorSystemConfiguration]

	// ActorSystemConfiguration ActorSystem 配置结构体
	ActorSystemConfiguration struct {
		Logger               log.Logger
		FutureDefaultTimeout time.Duration
		Hooks                []Hook
		Metrics              bool // 是否启用指标采集
		Network              ActorSystemNetworkConfiguration
		HTW                  HtwConfig
		HTWEnabled           bool // 是否启用层级时间轮（HTW），默认 true
	}

	ActorSystemNetworkConfigurator   = configurator.Configurator[*ActorSystemNetworkConfiguration]
	ActorSystemNetworkConfiguratorFN = configurator.FN[*ActorSystemNetworkConfiguration]
	ActorSystemNetworkOption         = configurator.Option[*ActorSystemNetworkConfiguration]

	// ActorSystemNetworkConfiguration ActorSystem 网络配置
	ActorSystemNetworkConfiguration struct {
		Network            string
		AdvertisedAddress  string
		BindAddress        string
		Server             processor.RPCServer
		Connector          processor.RPCConnProvider
		SerializerProvider provider.Provider[serializer.NameSerializer]
	}
)

func (c *ActorSystemConfiguration) WithHTW(htw HtwConfig) *ActorSystemConfiguration {
	c.HTW = htw
	return c
}

func WithActorSystemHTW(htw HtwConfig) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.WithHTW(htw)
	}
}

func (c *ActorSystemNetworkConfiguration) WithSerializerProvider(provider provider.Provider[serializer.NameSerializer]) *ActorSystemNetworkConfiguration {
	c.SerializerProvider = provider
	return nil
}

func WithActorSystemNetworkConfigurationSerializerProvider(provider provider.Provider[serializer.NameSerializer]) ActorSystemNetworkOption {
	return func(configuration *ActorSystemNetworkConfiguration) {
		configuration.WithSerializerProvider(provider)
	}
}

func (c *ActorSystemNetworkConfiguration) WithConnector(provider processor.RPCConnProvider) *ActorSystemNetworkConfiguration {
	c.Connector = provider
	return nil
}

func WithActorSystemNetworkConfigurationConnector(provider processor.RPCConnProvider) ActorSystemNetworkOption {
	return func(configuration *ActorSystemNetworkConfiguration) {
		configuration.WithConnector(provider)
	}
}

func (c *ActorSystemNetworkConfiguration) WithNetwork(network string) *ActorSystemNetworkConfiguration {
	c.Network = network
	return nil
}

func WithActorSystemNetworkConfigurationNetwork(network string) ActorSystemNetworkOption {
	return func(configuration *ActorSystemNetworkConfiguration) {
		configuration.WithNetwork(network)
	}
}

func (c *ActorSystemNetworkConfiguration) WithAdvertisedAddress(address string) *ActorSystemNetworkConfiguration {
	c.AdvertisedAddress = address
	return nil
}

func WithActorSystemNetworkConfigurationAdvertisedAddress(address string) ActorSystemNetworkOption {
	return func(configuration *ActorSystemNetworkConfiguration) {
		configuration.WithAdvertisedAddress(address)
	}
}

func (c *ActorSystemNetworkConfiguration) WithBindAddress(address string) *ActorSystemNetworkConfiguration {
	c.BindAddress = address
	return nil
}

func WithActorSystemNetworkConfigurationBindAddress(address string) ActorSystemNetworkOption {
	return func(configuration *ActorSystemNetworkConfiguration) {
		configuration.WithBindAddress(address)
	}
}

func (c *ActorSystemNetworkConfiguration) WithServer(server processor.RPCServer) *ActorSystemNetworkConfiguration {
	c.Server = server
	return nil
}

func WithActorSystemNetworkConfigurationServer(server processor.RPCServer) ActorSystemNetworkOption {
	return func(configuration *ActorSystemNetworkConfiguration) {
		configuration.WithServer(server)
	}
}

// WithNetwork 设置 Actor 系统网络配置
func (c *ActorSystemConfiguration) WithNetwork(configuration *ActorSystemNetworkConfiguration) *ActorSystemConfiguration {
	c.Network = *configuration
	return c
}

// WithNetworkWithConfigurators 设置 Actor 系统网络配置
func (c *ActorSystemConfiguration) WithNetworkWithConfigurators(configurators ...ActorSystemNetworkConfigurator) *ActorSystemConfiguration {
	nc := NewActorSystemNetworkConfiguration()
	for _, cc := range configurators {
		cc.Configure(nc)
	}
	return c.WithNetwork(nc)
}

// WithNetworkWithOptions 设置 Actor 系统网络配置
func (c *ActorSystemConfiguration) WithNetworkWithOptions(options ...ActorSystemNetworkOption) *ActorSystemConfiguration {
	nc := NewActorSystemNetworkConfiguration(options...)
	return c.WithNetwork(nc)
}

// WithActorSystemNetwork 设置 Actor 系统网络配置
func WithActorSystemNetwork(configuration *ActorSystemNetworkConfiguration) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.WithNetwork(configuration)
	}
}

// WithActorSystemNetworkWithConfigurators 设置 Actor 系统网络配置
func WithActorSystemNetworkWithConfigurators(configurators ...ActorSystemNetworkConfigurator) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.WithNetworkWithConfigurators(configurators...)
	}
}

// WithActorSystemNetworkWithOptions 设置 Actor 系统网络配置
func WithActorSystemNetworkWithOptions(options ...ActorSystemNetworkOption) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.WithNetworkWithOptions(options...)
	}
}

// WithLogger 设置日志器
func (c *ActorSystemConfiguration) WithLogger(logger log.Logger) *ActorSystemConfiguration {
	c.Logger = logger
	return c
}

// WithActorSystemLogger 设置 ActorSystem 日志器
func WithActorSystemLogger(logger log.Logger) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.Logger = logger
	}
}

// WithFutureDefaultTimeout 设置 Future 默认超时时间
func (c *ActorSystemConfiguration) WithFutureDefaultTimeout(timeout time.Duration) *ActorSystemConfiguration {
	c.FutureDefaultTimeout = timeout
	return c
}

// WithActorSystemFutureDefaultTimeoutFn 设置 Future 默认超时时间
func WithActorSystemFutureDefaultTimeoutFn(timeout time.Duration) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.FutureDefaultTimeout = timeout
	}
}

// WithHooks 设置 ActorSystem 钩子
func (c *ActorSystemConfiguration) WithHooks(hooks ...Hook) *ActorSystemConfiguration {
	c.Hooks = append(c.Hooks, hooks...)
	return c
}

// WithActorSystemHooks 设置 ActorSystem 钩子
func WithActorSystemHooks(hooks ...Hook) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.Hooks = append(c.Hooks, hooks...)
	}
}

// WithHookProviders 设置 ActorSystem 钩子提供者
func (c *ActorSystemConfiguration) WithHookProviders(hookProviders ...HookProvider) *ActorSystemConfiguration {
	for _, hookProvider := range hookProviders {
		c.Hooks = append(c.Hooks, hookProvider.hooks()...)
	}
	return c
}

// WithActorSystemHookProviders 设置 ActorSystem 钩子提供者
func WithActorSystemHookProviders(hookProviders ...HookProvider) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.WithHookProviders(hookProviders...)
	}
}

// WithMetrics 设置 ActorSystem 是否启用指标采集
func (c *ActorSystemConfiguration) WithMetrics(enable bool) *ActorSystemConfiguration {
	c.Metrics = enable
	return c
}

// WithHTWEnabled 设置是否启用层级时间轮（HTW）
// 如果禁用，Schedule 相关方法将不会工作，但可以节省 CPU 资源
func (c *ActorSystemConfiguration) WithHTWEnabled(enabled bool) *ActorSystemConfiguration {
	c.HTWEnabled = enabled
	return c
}

// WithActorSystemHTWEnabled 设置是否启用层级时间轮（HTW）
func WithActorSystemHTWEnabled(enabled bool) ActorSystemOption {
	return func(c *ActorSystemConfiguration) {
		c.WithHTWEnabled(enabled)
	}
}
