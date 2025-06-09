package vivid

import "github.com/kercylan98/vivid/src/configurator"

// NewStandardSupervisorConfiguration 创建新的StandardSupervisor配置实例
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *StandardSupervisorConfiguration: 配置实例
func NewStandardSupervisorConfiguration(options ...StandardSupervisorOption) *StandardSupervisorConfiguration {
	c := &StandardSupervisorConfiguration{}
	for _, option := range options {
		option(c)
	}
	return c
}

type (
	// StandardSupervisorConfigurator 配置器接口
	StandardSupervisorConfigurator = configurator.Configurator[*StandardSupervisorConfiguration]

	// StandardSupervisorConfiguratorFN 配置器函数类型
	StandardSupervisorConfiguratorFN = configurator.FN[*StandardSupervisorConfiguration]

	// StandardSupervisorOption 配置选项函数类型
	StandardSupervisorOption = configurator.Option[*StandardSupervisorConfiguration]

	// StandardSupervisorConfiguration StandardSupervisor配置结构体
	//
	// 所有字段均为私有，通过 GetXxx 方法获取值，通过 WithXxx 方法设置值。
	StandardSupervisorConfiguration struct {
		DirectiveProvider SupervisorDirectiveProvider
	}
)

// WithDirectiveProvider 设置 StandardSupervisorConfiguration 的 DirectiveProvider 字段
func (c *StandardSupervisorConfiguration) WithDirectiveProvider(directiveProvider SupervisorDirectiveProvider) *StandardSupervisorConfiguration {
	c.DirectiveProvider = directiveProvider
	return c
}

// WithStandardSupervisorDirectiveProvider 设置 StandardSupervisorConfiguration 的 DirectiveProvider 字段
func WithStandardSupervisorDirectiveProvider(directiveProvider SupervisorDirectiveProvider) StandardSupervisorOption {
	return func(c *StandardSupervisorConfiguration) {
		c.WithDirectiveProvider(directiveProvider)
	}
}
