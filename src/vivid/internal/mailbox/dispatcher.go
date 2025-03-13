package mailbox

func NewDispatcher() *Dispatcher {
	return new(Dispatcher)
}

type Dispatcher uint8

func (d Dispatcher) Dispatch(f func()) {
	go f()
}
