package gateway

import "github.com/kercylan98/vivid/pkg/vivid"

var _ vivid.Actor = (*gateway)(nil)

func New(configurators ...Configurator) vivid.Actor {
	return NewWithConfigurators(configurators...)
}

func NewWithConfigurators(configurators ...Configurator) vivid.Actor {
	c := NewConfiguration()
	for _, configurator := range configurators {
		configurator.Configure(c)
	}
	return NewFromConfig(c)
}

func NewWithOptions(options ...Option) vivid.Actor {
	c := NewConfiguration()
	for _, option := range options {
		option.Apply(c)
	}
	return NewFromConfig(c)
}

func NewFromConfig(configuration *Configuration) vivid.Actor {
	g := &gateway{
		config:   *configuration,
		sessions: make(map[string][]*sessionId),
	}
	return g
}

type gateway struct {
	config   Configuration
	sessions map[string][]*sessionId
}

func (g *gateway) Receive(context vivid.ActorContext) {
	switch m := context.Message().(type) {
	case *vivid.OnLaunch:
		g.onLaunch(context, m)
	case Transport:
		g.onTransport(context, m)
	case *sessionId:
		g.onSessionId(context, m)
	}
}

func (g *gateway) onLaunch(context vivid.ActorContext, m *vivid.OnLaunch) {
	for _, controller := range g.config.AssumeControllers {
		context.ActorOf(func() vivid.Actor {
			return newAssumeControllerActor(controller)
		}).WithConfigurators(vivid.ActorConfiguratorFN(func(c *vivid.ActorConfiguration) {
			if named, ok := controller.(AssumeControllerNamed); ok {
				c.WithName(named.GetName())
			}
		}))
	}
}

func (g *gateway) onTransport(context vivid.ActorContext, m Transport) {
	context.ActorOf(func() vivid.Actor {
		return newSessionActor(m, g.config.CodecProvider.Provide())
	}).Spawn()
}

func (g *gateway) onSessionId(context vivid.ActorContext, m *sessionId) {
	name := m.GetName()
	g.sessions[name] = append(g.sessions[name], m)
	context.Reply(m)
}
