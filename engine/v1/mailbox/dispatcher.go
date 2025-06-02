package mailbox

type Dispatcher interface {
    Dispatch(mailbox Mailbox)
}

type DispatcherFN func()

func (fn DispatcherFN) Dispatch(mailbox Mailbox) {
    fn()
}

type DispatcherProvider interface {
    Provide() Dispatcher
}

type DispatcherProviderFN func() Dispatcher

func (fn DispatcherProviderFN) Provide() Dispatcher {
    return fn()
}
