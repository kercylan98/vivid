package vivid

var _ Dispatcher = (*dispatcherImpl)(nil)

type Dispatcher interface {
	Dispatch(f func())
}
