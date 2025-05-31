package configurator

type Configurator[C any] interface {
    Configure(c C)
}

type FN[C any] func(c C)

func (f FN[C]) Configure(c C) {
    f(c)
}
