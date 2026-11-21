package actor

import (
	"context"
	"errors"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/virtual"
)

var (
	errVirtualActivationUnknown  = errors.New("virtual actor activation outcome is unknown")
	errVirtualActivationNotReady = errors.New("virtual actor activation is not ready")
)

type virtualActivationBackend interface {
	discover(context.Context, string, string) (clusterstore.VirtualOwner, error)
	activate(context.Context, virtualKey, clusterstore.VirtualOwner, uint64) (clusterstore.VirtualOwnership, error)
}

// activate 只请求初始化与就绪，不发送业务消息；回执未知时不换候选或重试认领。
func (c *virtualOwnershipClient) activate(ctx context.Context, key virtualKey, target clusterstore.VirtualOwner, generation uint64) (clusterstore.VirtualOwnership, error) {
	if _, err := clusterstore.ClaimVirtualOwnership(key.kind, key.name, target, 0); err != nil {
		return clusterstore.VirtualOwnership{}, err
	}
	if err := ctx.Err(); err != nil {
		return clusterstore.VirtualOwnership{}, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	ref, err := NewRef(target.Address, virtualDeliveryPath)
	if err != nil {
		return clusterstore.VirtualOwnership{}, err
	}
	request := &virtual.ActivationRequest{Group: c.control.group, Owner: target.InstanceID, Generation: generation, Kind: key.kind, Name: key.name, Timeout: ownershipTimeout(ctx)}
	future := c.control.system.Ask(ref, request, 0)
	stop := context.AfterFunc(ctx, func() { future.Close(ctx.Err()) })
	defer stop()
	result, err := future.Result()
	if err != nil {
		return clusterstore.VirtualOwnership{}, errors.Join(errVirtualActivationUnknown, err)
	}
	response, ok := result.(*virtual.ActivationResult)
	if !ok || !response.Valid() || response.Group != request.Group || response.Owner != request.Owner || response.Generation != request.Generation || response.Kind != key.kind || response.Name != key.name {
		return clusterstore.VirtualOwnership{}, errors.Join(errVirtualActivationUnknown, virtual.ErrInvalidActivation)
	}
	switch response.Status {
	case virtual.ActivationReady:
		return clusterstore.VirtualOwnership{Kind: key.kind, Name: key.name, Owner: target, Generation: response.CurrentGeneration}, nil
	case virtual.ActivationConflict:
		err = clusterstore.ErrVirtualOwnershipConflict
	case virtual.ActivationOverloaded:
		err = vivid.ErrorVirtualPendingMessagesFull
	case virtual.ActivationStopped:
		err = vivid.ErrorActorSystemStopped
	case virtual.ActivationInvalid:
		err = virtual.ErrInvalidActivation
	case virtual.ActivationFailed:
		err = vivid.ErrorActorSpawnFailed
	case virtual.ActivationUnknown:
		err = errVirtualActivationUnknown
	default:
		err = errVirtualActivationNotReady
	}
	return clusterstore.VirtualOwnership{}, err
}

func (m *virtualIngressMailbox) activate(envelope vivid.Envelop, request *virtual.ActivationRequest) {
	if !request.Valid() || envelope.Sender() == nil {
		return
	}
	v := m.coordinator
	snapshot := *request
	delivery := virtualRouteDelivery{activation: &snapshot, reply: envelope.Sender(), deadline: time.Now().Add(request.Timeout)}
	status := virtual.ActivationStatus(0)
	if envelope.System() || envelope.Receiver() == nil || envelope.Receiver().GetAddress() != v.system.Ref().GetAddress() || envelope.Receiver().GetPath() != virtualDeliveryPath {
		v.replyActivation(delivery, virtual.ActivationInvalid, 0)
		return
	}
	key := virtualKey{request.Kind, request.Name}
	provider := v.system.options.VirtualActorProviders[key.kind]
	v.mu.Lock()
	entry := v.entries[key]
	switch {
	case v.closed:
		status = virtual.ActivationStopped
	case v.gate == nil || v.gate.group != request.Group:
		status = virtual.ActivationInvalid
	case v.gate.owner.InstanceID != request.Owner || v.gate.owner.Address != v.system.Ref().GetAddress():
		status = virtual.ActivationConflict
	case provider == nil:
		status = virtual.ActivationUnavailable
	case v.pending >= v.options.PendingMessageLimit:
		status = virtual.ActivationOverloaded
	case entry == nil && request.Generation != 0:
		// 已有归属不授权重建本地实例，更不能把进程重启当作接管资格。
		status = virtual.ActivationUnavailable
	case entry != nil && entry.ownership != nil && (entry.ownership.retiring || entry.ownership.value.Owner != v.gate.owner || request.Generation != 0 && entry.ownership.value.Generation != request.Generation):
		status = virtual.ActivationConflict
	case entry != nil && entry.mailbox != nil && entry.mailbox.closing:
		status = virtual.ActivationUnavailable
	default:
		if entry == nil {
			entry = newVirtualActivation(key, provider)
			v.entries[key] = entry
			v.waiting = append(v.waiting, entry)
		}
		v.pending++
		if entry.creating || entry.mailbox == nil {
			entry.activating = append(entry.activating, delivery)
		} else {
			v.queueRouteLocked(entry, delivery)
		}
		v.startLocked()
	}
	v.mu.Unlock()
	if status != 0 {
		v.replyActivation(delivery, status, 0)
	}
}

func (v *virtualCoordinator) replyActivation(delivery virtualRouteDelivery, status virtual.ActivationStatus, generation uint64) {
	m := delivery.activation
	if status == virtual.ActivationReady {
		if !time.Now().Before(delivery.deadline) {
			status = virtual.ActivationUnavailable
		} else if m.Generation != 0 && m.Generation != generation {
			status = virtual.ActivationConflict
		}
	}
	if status != virtual.ActivationReady {
		generation = 0
	}
	result := &virtual.ActivationResult{Group: m.Group, Owner: m.Owner, Generation: m.Generation, Kind: m.Kind, Name: m.Name, Status: status, CurrentGeneration: generation}
	v.system.ResolveMailbox(delivery.reply).Enqueue(mailbox.NewEnvelop(false, v.system.ref, delivery.reply, result))
}
