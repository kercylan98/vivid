package vivid

import (
    "github.com/kercylan98/vivid/engine/v1/internal/processor"
)

type ActorSystem interface {
    ActorContext
}

func NewActorSystem() ActorSystem {
    sys := &actorSystem{
        registry: processor.NewRegistryWithConfigurators(),
    }

    sys.ActorContext = newActorContext(sys, sys.registry.GetUnitIdentifier(), nil, ActorProviderFN(func() Actor {
        return ActorFN(func(context ActorContext) {

        })
    }), NewActorConfiguration())

    return sys
}

type actorSystem struct {
    ActorContext
    registry processor.Registry
}
