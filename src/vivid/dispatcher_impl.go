package vivid

func newDispatcher() Dispatcher {
	return dispatcherImpl(0)
}

type dispatcherImpl uint8

func (d dispatcherImpl) Dispatch(f func()) {
	go f()
}
