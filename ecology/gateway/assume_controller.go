package gateway

import "github.com/kercylan98/vivid/pkg/vivid"

var _ vivid.Actor = (*assumeControllerActor)(nil)

type AssumeController interface {
	GetAssumeChannel() <-chan Transport
}

type AssumeControllerNamed interface {
	AssumeController
	GetName() string
}

func newAssumeControllerActor(controller AssumeController) *assumeControllerActor {
	return &assumeControllerActor{
		controller: controller,
	}
}

type assumeControllerActor struct {
	controller AssumeController
}

func (c *assumeControllerActor) Receive(context vivid.ActorContext) {
	switch m := context.Message().(type) {
	case *vivid.OnLaunch:
		c.onLaunch(context, m)
	case AssumeController:
		c.onAssumeController(context, m)
	}
}

func (c *assumeControllerActor) onLaunch(context vivid.ActorContext, m *vivid.OnLaunch) {
	context.Tell(context.Ref(), c.controller)
}

func (c *assumeControllerActor) onAssumeController(context vivid.ActorContext, m AssumeController) {
	context.Tell(context.Parent(), <-m.GetAssumeChannel())
}
