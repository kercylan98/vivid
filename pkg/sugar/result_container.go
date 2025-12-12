package sugar

type ResultContainer[T any] struct{}

func (ResultContainer[T]) Ok(v T) *Result[T] {
	return With(v, nil)
}

func (ResultContainer[T]) Error(err error) *Result[T] {
	return Err[T](err)
}

func (ResultContainer[T]) None() *Result[T] {
	return None[T]()
}
