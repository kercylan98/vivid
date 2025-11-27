package vivid

import "time"

type Actor interface {
	OnReceive(ctx ActorContext)
}

type ActorOption = func(options *ActorOptions)
type ActorOptions struct {
	Name              string        // Actor 名称
	Mailbox           Mailbox       // Actor 邮箱
	DefaultAskTimeout time.Duration // Actor 默认的 Ask 超时时间
}

func WithActorOptions(options ActorOptions) ActorOption {
	return func(opts *ActorOptions) {
		*opts = options
	}
}

func WithActorName(name string) ActorOption {
	return func(opts *ActorOptions) {
		opts.Name = name
	}
}

func WithActorMailbox(mailbox Mailbox) ActorOption {
	return func(opts *ActorOptions) {
		opts.Mailbox = mailbox
	}
}

func WithActorDefaultAskTimeout(timeout time.Duration) ActorOption {
	return func(opts *ActorOptions) {
		if timeout > 0 {
			opts.DefaultAskTimeout = timeout
		}
	}
}
