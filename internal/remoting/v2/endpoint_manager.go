package remoting

import (
	"sync"

	"github.com/kercylan98/vivid"
)

func newEndpointManager() *EndpointManager {
	return &EndpointManager{
		endpoints: make(map[string]vivid.ActorRef),
	}
}

type EndpointManager struct {
	endpoints map[string]vivid.ActorRef // endpoint-address -> endpoint-ref
	stopped   bool
	mu        sync.RWMutex
}

func (e *EndpointManager) registerEndpoint(address string, endpoint vivid.ActorRef) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.endpoints[address] = endpoint
}

func (e *EndpointManager) getEndpoint(address string) vivid.ActorRef {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.endpoints[address]
}

func (e *EndpointManager) unregisterEndpoint(address string, endpoint vivid.ActorRef) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if current, ok := e.endpoints[address]; ok && (endpoint == nil || current.Equals(endpoint)) {
		delete(e.endpoints, address)
	}
}

func (e *EndpointManager) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopped = true
}

func (e *EndpointManager) Stopped() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stopped
}

func (e *EndpointManager) ensureEndpoint(ctx vivid.ActorContext, address string, spawn func(address string) (vivid.ActorRef, error)) (vivid.ActorRef, error) {
	if e.Stopped() {
		return nil, vivid.ErrorActorSystemStopped
	}
	if endpoint := e.getEndpoint(address); endpoint != nil {
		return endpoint, nil
	}

	endpoint, err := spawn(address)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		ctx.Kill(endpoint, false, "endpoint manager stopped")
		return nil, vivid.ErrorActorSystemStopped
	}
	if current := e.endpoints[address]; current != nil {
		ctx.Kill(endpoint, false, "duplicate endpoint")
		return current, nil
	}
	e.endpoints[address] = endpoint
	return endpoint, nil
}

func (e *EndpointManager) emitToAddress(ctx vivid.ActorContext, address string, envelop vivid.Envelop, spawn func(address string) (vivid.ActorRef, error)) error {
	endpoint, err := e.ensureEndpoint(ctx, address, spawn)
	if err != nil {
		return err
	}
	ctx.Tell(endpoint, envelop)
	return nil
}

func (e *EndpointManager) EmitToEndpoint(ctx vivid.ActorContext, envelop vivid.Envelop, spawn func(address string) (vivid.ActorRef, error)) error {
	receiver := envelop.Receiver()
	if receiver == nil {
		return vivid.ErrorIllegalArgument.WithMessage("receiver is nil")
	}
	address := receiver.GetAddress()
	if address == "" {
		return vivid.ErrorIllegalArgument.WithMessage("receiver address is empty")
	}
	return e.emitToAddress(ctx, address, envelop, spawn)
}
