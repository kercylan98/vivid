package vivid

type Actor interface {
    Receive(context ActorContext)
}

type ActorFN func(context ActorContext)

func (fn ActorFN) Receive(context ActorContext) {
    fn(context)
}

type ActorProvider interface {
    Provide() Actor
}

type ActorProviderFN func() Actor

func (fn ActorProviderFN) Provide() Actor {
    return fn()
}
