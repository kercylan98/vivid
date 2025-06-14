package metrics

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/configurator"
)

// NewManagerConfiguration 创建新的Manager配置实例
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *ManagerConfiguration: 配置实例
func NewManagerConfiguration(options ...ManagerOption) *ManagerConfiguration {
	c := &ManagerConfiguration{
		Logger:     log.GetDefault(),
		MaxEntries: 1000,
	}
	for _, option := range options {
		option(c)
	}
	return c
}

type (
	// ManagerConfigurator 配置器接口
	ManagerConfigurator = configurator.Configurator[*ManagerConfiguration]

	// ManagerConfiguratorFN 配置器函数类型
	ManagerConfiguratorFN = configurator.FN[*ManagerConfiguration]

	// ManagerOption 配置选项函数类型
	ManagerOption = configurator.Option[*ManagerConfiguration]

	// ManagerConfiguration manager 配置结构体
	ManagerConfiguration struct {
		Logger     log.Logger // 日志器
		MaxEntries int        // 最大历史条目数
	}
)

func (c *ManagerConfiguration) WithLogger(logger log.Logger) *ManagerConfiguration {
	c.Logger = logger
	return c
}

func WithManagerLogger(logger log.Logger) ManagerOption {
	return func(c *ManagerConfiguration) {
		c.Logger = logger
	}
}

func (c *ManagerConfiguration) WithMaxEntries(maxEntries int) *ManagerConfiguration {
	c.MaxEntries = maxEntries
	return c
}

func WithManagerMaxEntries(maxEntries int) ManagerOption {
	return func(c *ManagerConfiguration) {
		c.MaxEntries = maxEntries
	}
}
