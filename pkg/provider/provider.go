// Package provider 提供了通用的提供者模式实现。
package provider

// Provider 定义了提供者接口，用于延迟创建或获取类型为 T 的值。
// 这是一个泛型接口，支持任意类型的值提供。
type Provider[T any] interface {
	// Provide 返回类型为 T 的值。
	// 实现者可以选择每次创建新实例或返回缓存的实例。
	Provide() T
}

// FN 是一个函数类型，实现了 Provider 接口。
// 它允许将任何返回类型为 T 的函数转换为 Provider。
type FN[T any] func() T

// Provide 实现 Provider 接口，调用底层函数并返回结果。
func (f FN[T]) Provide() T {
	return f()
}

type Param1Provider[P, T any] interface {
	Provide(P) T
}

type Param1FN[P, T any] func(P) T
