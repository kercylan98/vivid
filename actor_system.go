package vivid

import "time"

type ActorSystem interface {
	actorCore
}

type ActorSystemOption = func(options *ActorSystemOptions)
type ActorSystemOptions struct {
	DefaultAskTimeout time.Duration // Actor 默认的 Ask 超时时间
}

func WithActorSystemDefaultAskTimeout(timeout time.Duration) ActorSystemOption {
	return func(opts *ActorSystemOptions) {
		if timeout > 0 {
			opts.DefaultAskTimeout = timeout
		}
	}
}
