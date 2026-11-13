package remoting

import (
	"sync"

	"github.com/kercylan98/vivid"
)

func newEndpointManager() *EndpointManager {
	return &EndpointManager{
		endpoints: make(map[string]endpointRegistration),
		pending:   make(map[*pendingConnection]struct{}),
	}
}

type EndpointManager struct {
	endpoints map[string]endpointRegistration // endpoint-address -> endpoint-ref 与已投递数量
	pending   map[*pendingConnection]struct{}
	stopped   bool
	mu        sync.RWMutex
}

type endpointRegistration struct {
	ref        vivid.ActorRef
	dispatched uint64 // 已保留的投递数量，包含尚未入邮箱与尚未处理的消息
}

func (e *EndpointManager) unregisterEndpoint(address string, endpoint vivid.ActorRef) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if current, ok := e.endpoints[address]; ok && (endpoint == nil || current.ref.Equals(endpoint)) {
		delete(e.endpoints, address)
	}
}

func (e *EndpointManager) Stop() {
	e.mu.Lock()
	e.stopped = true
	pending := make([]*pendingConnection, 0, len(e.pending))
	for connection := range e.pending {
		pending = append(pending, connection)
	}
	e.mu.Unlock()
	for _, connection := range pending {
		connection.close()
	}
}

func (e *EndpointManager) Stopped() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stopped
}

// emitToAddress 将路由与投递计数一起提交，避免空闲回收越过尚未处理的消息。
func (e *EndpointManager) emitToAddress(ctx vivid.ActorContext, address string, message vivid.Message, spawn func(address string) (vivid.ActorRef, error)) error {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return vivid.ErrorRemotingStopped
	}
	current := e.endpoints[address]
	var duplicate vivid.ActorRef
	if current.ref == nil {
		// 创建 Actor 和通知均在锁外，避免生命周期回调重新访问管理器。
		e.mu.Unlock()
		created, err := spawn(address)
		if err != nil {
			return err
		}
		e.mu.Lock()
		if e.stopped {
			e.mu.Unlock()
			ctx.Kill(created, false, "endpoint manager stopped")
			return vivid.ErrorRemotingStopped
		}
		current = e.endpoints[address]
		if current.ref == nil {
			current.ref = created
		} else {
			duplicate = created
		}
	}
	current.dispatched++
	e.endpoints[address] = current
	e.mu.Unlock()
	if duplicate != nil {
		ctx.Kill(duplicate, false, "duplicate endpoint")
	}
	// 先保留投递名额，锁外入邮箱；即使入队延迟，端点也不能提前回收。
	ctx.Tell(current.ref, message)
	return nil
}

// retireEndpoint 仅移除已处理全部投递的同一端点；新投递与索引移除互斥。
func (e *EndpointManager) retireEndpoint(address string, endpoint vivid.ActorRef, received uint64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.endpoints[address]
	if e.stopped || received == 0 || current.ref == nil || !current.ref.Equals(endpoint) || current.dispatched != received {
		return false
	}
	delete(e.endpoints, address)
	return true
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
