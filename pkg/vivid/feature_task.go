package vivid

type FutureTask interface {
    Execute() Message
}

type FutureTaskFN func() Message

func (fn FutureTaskFN) Execute() Message {
    return fn()
}

type FutureTaskFailureHandler interface {
    OnFailure(ctx ActorContext, ref ActorRef, reason Message)
}

type FutureTaskFailureHandlerFN func(ctx ActorContext, ref ActorRef, reason Message)

func (fn FutureTaskFailureHandlerFN) OnFailure(ctx ActorContext, ref ActorRef, reason Message) {
    fn(ctx, ref, reason)
}
