package virtual

import (
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/bridge"
	"golang.org/x/sync/singleflight"
)

func newActivation(system bridge.VirtualActorSystem) *activation {
	return &activation{
		system: system,
		actors: make(map[string]vivid.ActorRef),
		ttl:    time.Minute * 10,
	}
}

type activation struct {
	system  bridge.VirtualActorSystem
	lock    sync.RWMutex
	actors  map[string]vivid.ActorRef
	loading singleflight.Group
	ttl     time.Duration
}

func (a *activation) deactivate(ref vivid.ActorRef) {
	a.lock.Lock()
	defer a.lock.Unlock()

	for key, r := range a.actors {
		if r.Equals(ref) {
			delete(a.actors, key)
			break
		}
	}
}

func (a *activation) activate(ctx vivid.ActorContext, identity *Identity) (vivid.ActorRef, error) {
	key := identity.String()

	a.lock.RLock()
	if ref, exist := a.actors[key]; exist {
		a.lock.RUnlock()
		return ref, nil
	}
	a.lock.RUnlock()

	provider := a.system.GetVirtualActorProvider(identity.kind)
	if provider == nil {
		return nil, vivid.ErrorVirtualActorProviderNotFound.WithMessage(identity.kind)
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	res, err, _ := a.loading.Do(key, func() (interface{}, error) {
		actor := newPassivatedActor(provider.Provide(), a.ttl)
		ref, err := ctx.ActorOf(actor, vivid.WithActorName(key))
		if err != nil {
			return nil, err
		}

		a.actors[key] = ref
		return ref, nil
	})

	if err != nil {
		return nil, err
	}
	return res.(vivid.ActorRef), nil
}
