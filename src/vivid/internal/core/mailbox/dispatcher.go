package mailbox

type Dispatcher interface {
	Dispatch(f func())
}

type DispatcherFN func(f func())

func (d DispatcherFN) Dispatch(f func()) {
	d(f)
}
