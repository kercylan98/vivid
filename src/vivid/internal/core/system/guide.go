package system

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

var _ actor.Actor = (*Guard)(nil)

func GuardProvider() actor.Provider {
	return actor.ProviderFN(func() actor.Actor {
		return &Guard{}
	})
}

type Guard struct{}

func (g *Guard) OnReceive(ctx actor.Context) {}
