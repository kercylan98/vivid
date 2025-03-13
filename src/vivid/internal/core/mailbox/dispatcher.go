package mailbox

type Dispatcher interface {
	Dispatch(f func())
}
