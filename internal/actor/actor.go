package actor

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
)

func init() {
	serialization.RegisterBuiltInInstanceType[vivid.ActorRef](new(Ref))
}
