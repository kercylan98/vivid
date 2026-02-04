package actor

import (
	"github.com/kercylan98/vivid"
)

func NewUselessActor() vivid.Actor {
	return vivid.ActorFN(func(ctx vivid.ActorContext) {})
}
