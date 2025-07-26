package provider

type Provider[T any] interface {
	Provide() T
}

type FN[T any] func() T

func (f FN[T]) Provide() T {
	return f()
}
