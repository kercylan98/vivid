package processor

import (
    "github.com/kercylan98/go-log/log"
    "github.com/kercylan98/vivid/engine/v1/processor"
    "github.com/kercylan98/vivid/src/configurator"
    "github.com/kercylan98/vivid/src/serializer"
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
    // 所有字段均为私有，通过 GetXxx 方法获取值，通过 WithXxx 方法设置值。
    RPCServerConfiguration struct {
        Logger            log.Logger                // 日志记录器
        Server            processor.RPCServer       // RPC 服务器核心
        Serializer        serializer.NameSerializer // 序列化器
        ReactorProvider   RPCConnReactorProvider    // 连接反应器提供器
        Network           string                    // 网络类型
        AdvertisedAddress string                    // 广播地址
        BindAddress       string                    // 绑定地址
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
