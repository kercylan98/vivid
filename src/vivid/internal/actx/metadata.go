package actx

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

var _ actor.MetadataContext = (*Metadata)(nil)

func NewMetadata(system actor.System, ref actor.Ref, parent actor.Ref, config *actor.Config) actor.MetadataContext {
	return &Metadata{
		system: system,
		ref:    ref,
		parent: parent,
		config: config,
	}
}

type Metadata struct {
	system actor.System  // Actor 所属的 System
	ref    actor.Ref     // Actor 本身的 ActorRef
	parent actor.Ref     // Actor 的父 ActorRef，如果没有父 Actor 则为 nil，那么视为根 Actor
	config *actor.Config // Actor 配置
}

func (m *Metadata) Config() *actor.Config {
	return m.config
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
