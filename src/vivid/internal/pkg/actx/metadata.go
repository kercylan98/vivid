package actx

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

var _ actor.MetadataContext = (*Metadata)(nil)

func NewMetadata(system actor.System, ref actor.Ref, parent actor.Ref) actor.MetadataContext {
	return &Metadata{
		ref:    ref,
		parent: parent,
	}
}

type Metadata struct {
	system actor.System
	ref    actor.Ref // Actor 本身的 ActorRef
	parent actor.Ref // Actor 的父 ActorRef，如果没有父 Actor 则为 nil，那么视为根 Actor
}

func (m *Metadata) System() actor.System {
	return m.system
}

func (m *Metadata) Ref() actor.Ref {
	return m.ref
}

func (m *Metadata) Parent() actor.Ref {
	return m.parent
}
