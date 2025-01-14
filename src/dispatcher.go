package vivid

var (
	_                  Dispatcher = (*defaultDispatcher)(nil)
	_defaultDispatcher Dispatcher = &defaultDispatcher{}
)

func defaultDispatcherProvider() Dispatcher {
	return _defaultDispatcher
}

type Dispatcher interface {
	Dispatch(handler func())
}

type DispatcherProvider interface {
	Provide() Dispatcher
}

type DispatcherProviderFn func() Dispatcher

func (f DispatcherProviderFn) Provide() Dispatcher {
	return f()
}

type defaultDispatcher struct{}

func (d *defaultDispatcher) Dispatch(handler func()) {
	go handler()
}
