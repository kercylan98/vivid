// Package configurator 提供了通用的配置器模式实现。
//
// 配置器模式允许以灵活的方式配置对象，支持链式调用和组合配置。
// 该包提供了泛型配置器接口，适用于各种配置场景。
package configurator

// Configurator 定义了泛型配置器接口。
//
// 配置器模式是一种设计模式，用于以灵活的方式配置对象。
// 它允许将配置逻辑封装在独立的配置器中，支持组合和重用。
//
// 类型参数 C 是要配置的对象类型。
//
// 典型用法：
//   configurator := ConfiguratorFN(func(config *MyConfig) {
//       config.Field = "value"
//   })
//   configurator.Configure(myConfig)
type Configurator[C any] interface {
	// Configure 对给定的配置对象进行配置。
	//
	// 参数 c 是要配置的对象，配置器会修改其属性或状态。
	Configure(c C)
}

// FN 是 Configurator 接口的函数式实现。
//
// 允许使用函数直接实现配置器，简化了配置逻辑的编写。
// 类型参数 C 是要配置的对象类型。
type FN[C any] func(c C)

// Configure 实现 Configurator 接口的 Configure 方法。
func (f FN[C]) Configure(c C) {
	f(c)
}
