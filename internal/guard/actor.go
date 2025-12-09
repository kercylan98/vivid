package guard

import "github.com/kercylan98/vivid"

var (
	_ vivid.Actor = &Actor{}
)

func NewActor(guardClosedSignal chan struct{}) *Actor {
	return &Actor{
		guardClosedSignal: guardClosedSignal,
	}
}

type Actor struct {
	guardClosedSignal chan struct{}
}

func (a *Actor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnKilled:
		a.onKilled(ctx, msg)
	}
}

func (a *Actor) onKilled(ctx vivid.ActorContext, msg *vivid.OnKilled) {
	if msg.Ref.Equals(ctx.Ref()) {
		close(a.guardClosedSignal)
	}
}
