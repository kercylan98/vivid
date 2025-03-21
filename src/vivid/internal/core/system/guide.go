package system

import (
	"context"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

var _ actor.Actor = (*Guard)(nil)

func GuardProvider(cancel context.CancelFunc) actor.Provider {
	return actor.ProviderFN(func() actor.Actor {
		return &Guard{cancel: cancel}
	})
}

type Guard struct {
	cancel context.CancelFunc
}

func (g *Guard) OnReceive(ctx actor.Context) {
	switch ctx.MessageContext().Message().(type) {
	case *actor.OnKilled:
		g.cancel()
	}
}
