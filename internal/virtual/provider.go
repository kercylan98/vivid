package virtual

import (
	"github.com/kercylan98/vivid"
)

var (
	_ vivid.ActorProvider = (*Provider)(nil)
)

// NewProvider creates a new virtual actor provider that uses the given factory function.
func NewProvider(factory func() vivid.Actor) *Provider {
	return &Provider{
		factory: factory,
	}
}

type Provider struct {
	factory func() vivid.Actor
}

func (p *Provider) Provide() vivid.Actor {
	return p.factory()
}
