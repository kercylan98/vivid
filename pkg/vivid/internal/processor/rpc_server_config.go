package processor

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/pkg/configurator"
	"github.com/kercylan98/vivid/pkg/serializer"
	"github.com/kercylan98/vivid/pkg/vivid/processor"
)

// NewRPCServerConfiguration 创建新的RPCServer配置实例
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *RPCServerConfiguration: 配置实例
func NewRPCServerConfiguration(options ...RPCServerOption) *RPCServerConfiguration {
	c := &RPCServerConfiguration{}
	for _, option := range options {
		option(c)
	}
	return c
}

type (
	// RPCServerConfigurator 配置器接口
	RPCServerConfigurator = configurator.Configurator[*RPCServerConfiguration]

	// RPCServerConfiguratorFN 配置器函数类型
	RPCServerConfiguratorFN = configurator.FN[*RPCServerConfiguration]

	// RPCServerOption 配置选项函数类型
	RPCServerOption = configurator.Option[*RPCServerConfiguration]

	// RPCServerConfiguration RPCServer配置结构体
	//
	// 包含 RPC 服务器运行所需的所有配置参数。
	// 所有字段均为私有，通过 GetXxx 方法获取值，通过 WithXxx 方法设置值。
	RPCServerConfiguration struct {
		// Logger 日志记录器，用于记录 RPC 服务器运行时的日志信息。
		// 包括连接建立、消息传输、错误处理等运行时日志。
		// 默认值：log.GetDefault()
		// 注意：建议在生产环境中使用结构化日志记录器以便于问题排查
		Logger log.Logger

		// Server RPC 服务器核心实现，负责处理底层网络连接和消息传输。
		// 提供连接监听、消息收发、连接管理等核心功能。
		// 必填项：此字段必须设置，否则 RPC 服务器无法正常工作
		// 注意：不同的实现可能有不同的性能特征和功能支持
		Server processor.RPCServer

		// Serializer 序列化器，用于消息的序列化和反序列化。
		// 负责将消息对象转换为字节流进行网络传输，以及反向转换。
		// 必填项：此字段必须设置，否则无法正确处理消息
		// 注意：序列化器的选择会影响性能和兼容性，建议选择高效且跨语言的格式
		Serializer serializer.NameSerializer

		// ReactorProvider 连接反应器提供器，用于处理连接上的消息事件。
		// 提供消息处理器实例，负责具体的消息路由和业务逻辑处理。
		// 必填项：此字段必须设置，否则无法处理接收到的消息
		// 注意：反应器的实现会直接影响消息处理的性能和正确性
		ReactorProvider RPCConnReactorProvider

		// Network 网络类型，指定服务器监听的网络协议。
		// 支持的值通常包括 "tcp"、"tcp4"、"tcp6"、"unix" 等。
		// 默认值：建议设置为 "tcp"
		// 注意：网络类型的选择应该与客户端保持一致
		Network string

		// AdvertisedAddress 广播地址，向其他节点公布的服务器地址。
		// 其他节点将使用此地址建立到该服务器的连接。
		// 必填项：此字段必须设置为其他节点可访问的地址
		// 注意：应该是外部可访问的地址，而不是内部绑定地址
		AdvertisedAddress string

		// BindAddress 绑定地址，服务器实际监听的网络地址。
		// 格式通常为 "host:port"，如 "0.0.0.0:8080" 或 ":8080"。
		// 必填项：此字段必须设置，指定服务器监听的具体地址和端口
		// 注意：使用 "0.0.0.0" 可以监听所有网络接口，生产环境中应谨慎使用
		BindAddress string
	}
)

func (c *RPCServerConfiguration) WithLogger(logger log.Logger) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.Logger = logger
	}
}

func WithRPRCServerLogger(logger log.Logger) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.Logger = logger
	}
}

func (c *RPCServerConfiguration) WithRPCServer(server processor.RPCServer) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.Server = server
	}
}

func WithRPCServer(server processor.RPCServer) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.Server = server
	}
}

func (c *RPCServerConfiguration) WithSerializer(serializer serializer.NameSerializer) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.Serializer = serializer
	}
}

func WithRPCServerSerializer(serializer serializer.NameSerializer) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.Serializer = serializer
	}
}

func (c *RPCServerConfiguration) WithReactorProvider(provider RPCConnReactorProvider) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.ReactorProvider = provider
	}
}

func WithRPCServerReactorProvider(provider RPCConnReactorProvider) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.ReactorProvider = provider
	}
}

func (c *RPCServerConfiguration) WithNetwork(network string) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.Network = network
	}
}

func WithRPCServerNetwork(network string) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.Network = network
	}
}

func (c *RPCServerConfiguration) WithAdvertisedAddress(address string) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.AdvertisedAddress = address
	}
}

func WithRPCServerAdvertisedAddress(address string) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.AdvertisedAddress = address
	}
}

func (c *RPCServerConfiguration) WithBindAddress(address string) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.BindAddress = address
	}
}

func WithRPCServerBindAddress(address string) RPCServerOption {
	return func(c *RPCServerConfiguration) {
		c.BindAddress = address
	}
}
