package configurator

type Option[C any] func(C)

func (o Option[C]) With(f func(C)) Option[C] {
    return func(c C) {
        f(c)
        o(c)
    }
}
