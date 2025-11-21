package guard

import "github.com/kercylan98/vivid"

var (
	_ vivid.Actor = &GuardActor{}
)

func NewGuardActor() *GuardActor {
	return &GuardActor{}
}

type GuardActor struct {
}

func (a *GuardActor) OnReceive(ctx vivid.ActorContext) {
	// Do nothing
}
