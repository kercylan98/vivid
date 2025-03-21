package vivid

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

type (
	OnLaunch = actor.OnLaunch
)

type OnKill struct {
	m *actor.OnKill
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

type OnDead struct {
	m *actor.OnDead
}

func (o *OnDead) Ref() ActorRef {
	return o.m.Ref
}
