package vivid

type WatchHandler interface {
	Handle(ctx ActorContext, stopped OnWatchStopped)
}

type WatchHandlerFn func(ctx ActorContext, stopped OnWatchStopped)

func (fn WatchHandlerFn) Handle(ctx ActorContext, stopped OnWatchStopped) {
	fn(ctx, stopped)
}
