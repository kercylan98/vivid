package vivid

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

type OnKill struct {
	m actor.OnKill
}

func (o *OnKill) Operator() ActorRef {
	return o.m.Operator
}

func (o *OnKill) Poison() bool {
	return o.m.Poison
}

func (o *OnKill) Reason() string {
	return o.m.Reason
}
