package processor

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/pkg/configurator"
	"github.com/kercylan98/vivid/pkg/provider"
	"github.com/kercylan98/vivid/pkg/serializer"
)

// NewRPCUnitConfiguration 创建新的RPCUnit配置实例
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *RPCUnitConfiguration: 配置实例
func NewRPCUnitConfiguration(options ...RPCUnitOption) *RPCUnitConfiguration {
	c := &RPCUnitConfiguration{
		Logger:    log.GetDefault(),
		BatchSize: 1000,
		FailRetry: 10,
	}
	for _, option := range options {
		option(c)
	}
	return c
}

type (
	// RPCUnitConfigurator 配置器接口
	RPCUnitConfigurator = configurator.Configurator[*RPCUnitConfiguration]

	// RPCUnitConfiguratorFN 配置器函数类型
	RPCUnitConfiguratorFN = configurator.FN[*RPCUnitConfiguration]

	// RPCUnitOption 配置选项函数类型
	RPCUnitOption = configurator.Option[*RPCUnitConfiguration]

	// RPCUnitConfiguration RPCUnit配置结构体
	RPCUnitConfiguration struct {
		Logger             log.Logger                                   // 日志记录器
		SerializerProvider provider.Provider[serializer.NameSerializer] // 名称序列化器
		BatchSize          int                                          // 批量处理大小
		FailRetry          int                                          // 失败重试次数
	}
)

func (c *RPCUnitConfiguration) WithLogger(logger log.Logger) *RPCUnitConfiguration {
	c.Logger = logger
	return c
}

func WithRPCUnitLogger(logger log.Logger) RPCUnitOption {
	return func(configuration *RPCUnitConfiguration) {
		configuration.WithLogger(logger)
	}
}

func (c *RPCUnitConfiguration) WithSerializerProvider(provider provider.Provider[serializer.NameSerializer]) *RPCUnitConfiguration {
	c.SerializerProvider = provider
	return c
}

func WithRPCUnitSerializerProvider(provider provider.Provider[serializer.NameSerializer]) RPCUnitOption {
	return func(configuration *RPCUnitConfiguration) {
		configuration.WithSerializerProvider(provider)
	}
}

func (c *RPCUnitConfiguration) WithBatchSize(batchSize int) *RPCUnitConfiguration {
	c.BatchSize = batchSize
	return c
}

func WithRPCUnitBatchSize(batchSize int) RPCUnitOption {
	return func(configuration *RPCUnitConfiguration) {
		configuration.WithBatchSize(batchSize)
	}
}

func (c *RPCUnitConfiguration) WithFailRetry(failRetry int) *RPCUnitConfiguration {
	c.FailRetry = failRetry
	return c
}

func WithRPCUnitFailRetry(failRetry int) RPCUnitOption {
	return func(configuration *RPCUnitConfiguration) {
		configuration.WithFailRetry(failRetry)
	}
}
