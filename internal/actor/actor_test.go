package actor

import (
	"github.com/kercylan98/vivid"
)

var ExportNewReplaceEnvelop = newReplacedEnvelop

func NewUselessActor() vivid.Actor {
	return vivid.ActorFN(func(ctx vivid.ActorContext) {})
}
