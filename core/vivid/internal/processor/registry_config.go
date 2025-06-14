// Package processor 提供了注册表配置功能。
package processor

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/core/vivid/processor"
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
		Logger: log.GetDefault(),
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
		// Logger 日志记录器，用于记录注册表运行时的日志信息。
		// 包括单元注册、注销、RPC 连接状态、错误信息等运行时日志。
		// 默认值：log.GetDefault()
		// 注意：建议在生产环境中使用结构化日志记录器以便于日志分析
		Logger log.Logger

		// Daemon 守护单元，当无法找到指定的处理单元时将使用此单元处理消息。
		// 作为兜底机制，确保系统在部分单元不可用时仍能继续运行。
		// 默认值：nil（未设置守护单元时，查找失败会返回 ErrDaemonUnitNotSet 错误）
		// 注意：守护单元应该能够处理任意类型的消息，建议实现通用的错误处理逻辑
		Daemon Unit

		// RPCServer RPC 服务器实例，用于处理远程处理单元的连接和通信。
		// 当注册表需要与其他节点进行通信时，会启动此服务器监听远程连接。
		// 默认值：nil（不启用 RPC 服务器，仅支持本地处理单元）
		// 注意：启用后注册表会自动管理服务器的生命周期，包括启动、停止和连接管理
		RPCServer *RPCServer

		// RPCClientProvider RPC 客户端连接提供器，用于建立到远程节点的连接。
		// 当需要访问远程处理单元时，注册表会使用此提供器获取连接。
		// 默认值：nil（使用内置的连接查找机制，通过 RPCServer 获取现有连接）
		// 注意：自定义提供器可以实现连接池、负载均衡等高级功能
		RPCClientProvider processor.RPCConnProvider

		// RPCUnitConfigurator RPC 单元配置器，用于配置远程处理单元的行为。
		// 包括序列化设置、超时参数、重试策略等远程单元特定的配置。
		// 默认值：nil（使用默认的 RPC 单元配置）
		// 注意：此配置器会应用到所有通过 RPC 访问的远程处理单元
		RPCUnitConfigurator RPCUnitConfigurator
	}
)

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

// WithRPCServer 设置 RPC 服务器实例。
// 该方法返回配置实例本身，支持链式调用。
// rpcServer 参数指定要使用的 RPC 服务器。
func (c *RegistryConfiguration) WithRPCServer(rpcServer *RPCServer) *RegistryConfiguration {
	c.RPCServer = rpcServer
	return c
}

// WithRegistryRPCServer 创建设置 RPC 服务器的配置选项。
// 返回一个可用于 NewRegistryConfiguration 的配置选项函数。
// rpcServer 参数指定要设置的 RPC 服务器。
func WithRegistryRPCServer(rpcServer *RPCServer) RegistryOption {
	return func(configuration *RegistryConfiguration) {
		configuration.WithRPCServer(rpcServer)
	}
}

// WithRPCClientProvider 设置 RPC 客户端连接提供器。
// 该方法返回配置实例本身，支持链式调用。
// rpcClientProvider 参数指定要使用的 RPC 客户端连接提供器。
func (c *RegistryConfiguration) WithRPCClientProvider(rpcClientProvider processor.RPCConnProvider) *RegistryConfiguration {
	c.RPCClientProvider = rpcClientProvider
	return c
}

// WithRPCClientProvider 创建设置 RPC 客户端连接提供器的配置选项。
// 返回一个可用于 NewRegistryConfiguration 的配置选项函数。
// rpcClientProvider 参数指定要设置的 RPC 客户端连接提供器。
func WithRPCClientProvider(rpcClientProvider processor.RPCConnProvider) RegistryOption {
	return func(configuration *RegistryConfiguration) {
		configuration.WithRPCClientProvider(rpcClientProvider)
	}
}

// WithRPCUnitConfigurator 设置 RPC 单元配置器。
// 该方法返回配置实例本身，支持链式调用。
// rpcUnitConfigurator 参数指定要使用的 RPC 单元配置器。
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
