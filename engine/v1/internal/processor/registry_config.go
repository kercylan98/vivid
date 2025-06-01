// Package processor 提供了注册表配置功能。
package processor

import (
    "github.com/kercylan98/go-log/log"
    "github.com/kercylan98/vivid/engine/v1/processor"
    "github.com/kercylan98/vivid/src/configurator"
)

const (
    // onlyLocalAddress 表示仅本地地址的常量
    onlyLocalAddress = "localhost"
)

// NewRegistryConfiguration 创建新的注册表配置实例。
// 使用选项模式支持灵活的配置，提供合理的默认值。
// options 参数允许自定义配置选项。
func NewRegistryConfiguration(options ...RegistryOption) *RegistryConfiguration {
    c := &RegistryConfiguration{
        RootUnitIdentifier: newUnitIdentifier(onlyLocalAddress, "/"),
        Logger:             log.GetDefault(),
    }
    for _, option := range options {
        option(c)
    }
    return c
}

type (
    // RegistryConfigurator 定义了注册表配置器接口
    RegistryConfigurator = configurator.Configurator[*RegistryConfiguration]

    // RegistryConfiguratorFN 定义了注册表配置器函数类型
    RegistryConfiguratorFN = configurator.FN[*RegistryConfiguration]

    // RegistryOption 定义了注册表配置选项类型
    RegistryOption = configurator.Option[*RegistryConfiguration]

    // RegistryConfiguration 定义了注册表的配置结构。
    // 包含注册表运行所需的所有配置参数。
    RegistryConfiguration struct {
        RootUnitIdentifier  UnitIdentifier            // 顶级单元标识符，用作注册表的根节点
        Logger              log.Logger                // 日志记录器，用于记录注册表运行日志
        Daemon              Unit                      // 守护单元
        RPCServer           *RPCServer                // RPC 服务
        RPCClientProvider   processor.RPCConnProvider // RPC 客户端提供器
        RPCUnitConfigurator RPCUnitConfigurator       // RPC 单元配置器
    }
)

// WithUnitIdentifier 设置顶级单元标识符。
// 该方法返回配置实例本身，支持链式调用。
// unitIdentifier 参数指定注册表的根单元标识符。
func (c *RegistryConfiguration) WithUnitIdentifier(unitIdentifier UnitIdentifier) *RegistryConfiguration {
    c.RootUnitIdentifier = unitIdentifier
    return c
}

// WithUnitIdentifier 创建设置单元标识符的配置选项。
// 返回一个可用于 NewRegistryConfiguration 的配置选项函数。
// unitIdentifier 参数指定要设置的单元标识符。
func WithUnitIdentifier(unitIdentifier UnitIdentifier) RegistryOption {
    return func(configuration *RegistryConfiguration) {
        configuration.WithUnitIdentifier(unitIdentifier)
    }
}

// WithLogger 设置日志记录器。
// 该方法返回配置实例本身，支持链式调用。
// logger 参数指定要使用的日志记录器。
func (c *RegistryConfiguration) WithLogger(logger log.Logger) *RegistryConfiguration {
    c.Logger = logger
    return c
}

// WithLogger 创建设置日志记录器的配置选项。
// 返回一个可用于 NewRegistryConfiguration 的配置选项函数。
// logger 参数指定要设置的日志记录器。
func WithLogger(logger log.Logger) RegistryOption {
    return func(configuration *RegistryConfiguration) {
        configuration.WithLogger(logger)
    }
}

// WithDaemon 设置守护单元。
// 该方法返回配置实例本身，支持链式调用。
// daemon 参数指定要使用的守护单元。
func (c *RegistryConfiguration) WithDaemon(daemon Unit) *RegistryConfiguration {
    c.Daemon = daemon
    return c
}

// WithDaemon 创建设置守护单元的配置选项。
// 返回一个可用于 NewRegistryConfiguration 的配置选项函数。
// daemon 参数指定要设置的守护单元。
func WithDaemon(daemon Unit) RegistryOption {
    return func(configuration *RegistryConfiguration) {
        configuration.WithDaemon(daemon)
    }
}

// WithRPCServer 设置 RPC 服务配置。
func (c *RegistryConfiguration) WithRPCServer(rpcServer *RPCServer) *RegistryConfiguration {
    c.RPCServer = rpcServer
    return c
}

// WithRPCClientProvider 设置 RPC 客户端提供器。
func (c *RegistryConfiguration) WithRPCClientProvider(rpcClientProvider processor.RPCConnProvider) *RegistryConfiguration {
    c.RPCClientProvider = rpcClientProvider
    return c
}

// WithRPCUnitConfigurator 设置 RPC 单元配置器。
func (c *RegistryConfiguration) WithRPCUnitConfigurator(rpcUnitConfigurator RPCUnitConfigurator) *RegistryConfiguration {
    c.RPCUnitConfigurator = rpcUnitConfigurator
    return c
}

// WithRPCUnitConfigurator 创建设置 RPC 单元配置器的配置选项。
// 返回一个可用于 NewRegistryConfiguration 的配置选项函数。
// rpcUnitConfigurator 参数指定要设置的 RPC 单元配置器。
func WithRPCUnitConfigurator(rpcUnitConfigurator RPCUnitConfigurator) RegistryOption {
    return func(configuration *RegistryConfiguration) {
        configuration.WithRPCUnitConfigurator(rpcUnitConfigurator)
    }
}
