// Package configurator 提供了通用的配置器模式实现。
 //
// 配置器模式是一种设计模式，用于以灵活的方式配置对象。
// 支持函数式编程和组合模式，提供类型安全的配置能力。
package configurator

// Configurator 定义了泛型配置器接口。
//
// 配置器模式的核心接口，用于统一配置对象的方式。
// 类型参数 C 是要配置的对象类型。
type Configurator[C any] interface {
	// Configure 对给定的配置对象进行配置。
	Configure(c C)
}

// FN 是 Configurator 接口的函数式实现。
//
// 允许使用函数直接实现配置器，简化配置逻辑的编写。
type FN[C any] func(c C)

// Configure 实现 Configurator 接口。
func (f FN[C]) Configure(c C) {
	f(c)
}
