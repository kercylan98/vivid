package actor

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/kercylan98/vivid/pkg/ves"
)

const virtualDeliveryPath = "/@virtual-delivery"

var errVirtualDeliveryUnknown = errors.New("virtual actor delivery admission is unknown")

type virtualRoutingIdentity interface {
	routingIdentity() ([32]byte, clusterstore.VirtualOwner)
}

func (c *virtualOwnershipClient) routingIdentity() ([32]byte, clusterstore.VirtualOwner) {
	return c.control.group, c.owner
}

func (c *virtualOwnershipClient) lookup(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, bool, error) {
	name, err := clusterstore.VirtualOwnershipKey(key.kind, key.name)
	if err != nil {
		return clusterstore.VirtualOwnership{}, false, err
	}
	result, err := c.control.call(ctx, clusterstore.RaftControlGet, clusterstore.Command{}, name)
	if err != nil {
		return clusterstore.VirtualOwnership{}, false, err
	}
	return clusterstore.ReadVirtualOwnership(key.kind, key.name, result.Record())
}

// deliver 只发送一次；回执丢失不能证明尚未入业务邮箱，调用方不得因此重新投递。
// 明确的拒绝表示本次没有入队，后续可在原预算内重新查询；此处不缓存地址或自动重试。
func (c *virtualOwnershipClient) deliver(ctx context.Context, ownership clusterstore.VirtualOwnership, sender vivid.ActorRef, message vivid.Message) (virtual.DeliveryStatus, error) {
	if _, err := ownership.Release(); err != nil || sender == nil || sender.IsVirtual() {
		return 0, vivid.ErrorIllegalArgument
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	request := &virtual.RoutedDelivery{Group: c.control.group, Owner: ownership.Owner.InstanceID, Generation: ownership.Generation, Kind: ownership.Kind, Name: ownership.Name, Sender: sender.String(), Message: message}
	if !request.Valid() {
		return 0, virtual.ErrInvalidRoutedDelivery
	}
	ref, err := NewRef(ownership.Owner.Address, virtualDeliveryPath)
	if err != nil {
		return 0, err
	}
	future := c.control.system.Ask(ref, request, 0)
	stop := context.AfterFunc(ctx, func() { future.Close(ctx.Err()) })
	defer stop()
	result, err := future.Result()
	if err != nil {
		return 0, errors.Join(errVirtualDeliveryUnknown, err)
	}
	response, ok := result.(*virtual.DeliveryResult)
	if !ok || response == nil || response.Group != request.Group || response.Owner != request.Owner || response.Generation != request.Generation || response.Kind != request.Kind || response.Name != request.Name || response.Status < virtual.DeliveryAccepted || response.Status > virtual.DeliveryInvalid {
		return 0, errors.Join(errVirtualDeliveryUnknown, virtual.ErrInvalidRoutedDelivery)
	}
	return response.Status, nil
}

// virtualIngressMailbox 只进入有界协调队列，不建立 Actor、常驻 goroutine 或目标地址缓存。
type virtualIngressMailbox struct{ coordinator *virtualCoordinator }

func (*virtualIngressMailbox) Pause()         {}
func (*virtualIngressMailbox) Resume()        {}
func (*virtualIngressMailbox) IsPaused() bool { return false }

func (m *virtualIngressMailbox) Enqueue(envelope vivid.Envelop) {
	v := m.coordinator
	request, ok := envelope.Message().(*virtual.RoutedDelivery)
	if !ok || !request.Valid() || envelope.Sender() == nil {
		v.system.TellSelf(ves.DeathLetterEvent{Envelope: envelope, Time: time.Now()})
		return
	}
	delivery := virtualRouteDelivery{request: *request, reply: envelope.Sender()}
	sender, err := ParseRef(request.Sender)
	if err != nil || envelope.System() || envelope.Receiver() == nil || envelope.Receiver().GetAddress() != v.system.Ref().GetAddress() || envelope.Receiver().GetPath() != virtualDeliveryPath {
		v.replyRoute(delivery, virtual.DeliveryInvalid)
		return
	}
	delivery.sender = sender
	v.mu.Lock()
	status := virtual.DeliveryStatus(0)
	key := virtualKey{request.Kind, request.Name}
	entry := v.entries[key]
	switch {
	case v.closed:
		status = virtual.DeliveryStopped
	case v.gate == nil || v.gate.group != request.Group:
		status = virtual.DeliveryInvalid
	case entry == nil || entry.creating || entry.mailbox == nil || entry.mailbox.closing:
		status = virtual.DeliveryUnavailable
	case entry.ownership == nil || entry.ownership.value.Owner != v.gate.owner || v.gate.owner.Address != v.system.Ref().GetAddress() || entry.ownership.value.Generation != request.Generation || entry.ownership.value.Owner.InstanceID != request.Owner:
		status = virtual.DeliveryStale
	case v.pending >= v.options.PendingMessageLimit:
		status = virtual.DeliveryOverloaded
	default:
		state := entry.ownership
		state.deliveries = append(state.deliveries, delivery)
		v.pending++
		if !state.checking {
			state.checking = true
			v.checking = append(v.checking, &virtualRouteBatch{entry: entry, state: state})
		}
		v.startLocked()
	}
	v.mu.Unlock()
	if status != 0 {
		v.replyRoute(delivery, status)
	}
}

type virtualRouteDelivery struct {
	request virtual.RoutedDelivery
	sender  vivid.ActorRef
	reply   vivid.ActorRef
}

type virtualRouteBatch struct {
	entry      *virtualActivation
	state      *virtualOwnershipState
	deliveries []virtualRouteDelivery
}

func (v *virtualCoordinator) routeAvailable(batch *virtualRouteBatch) bool {
	entry := batch.entry
	return !v.closed && v.entries[entry.key] == entry && entry.ownership == batch.state && !entry.creating && entry.mailbox != nil && !entry.mailbox.closing && atomic.LoadInt32(&entry.mailbox.ctx.state) == running
}

// checkRoute 只复核开始时已经取出的批次；复核开始后的新消息必须进入下一次确认。
// 多数派确认结束到业务执行之间仍可能失权，不能将此准入描述为租约或外部副作用隔离。
func (v *virtualCoordinator) checkRoute(batch *virtualRouteBatch) {
	v.mu.Lock()
	available := v.routeAvailable(batch)
	v.mu.Unlock()
	var err error
	if available {
		ctx, cancel := context.WithTimeout(v.gate.ctx, v.gate.timeout)
		err = v.gate.backend.validate(ctx, batch.state.value)
		cancel()
	}
	v.mu.Lock()
	status := virtual.DeliveryUnavailable
	var stopping *virtualMailbox
	if v.closed {
		status = virtual.DeliveryStopped
	} else if errors.Is(err, clusterstore.ErrVirtualOwnershipConflict) {
		status = virtual.DeliveryStale
		if v.entries[batch.entry.key] == batch.entry && batch.entry.ownership == batch.state && batch.entry.mailbox != nil {
			stopping = batch.entry.mailbox
			stopping.closing = true
		}
	} else if available && err == nil && v.routeAvailable(batch) {
		status = virtual.DeliveryAccepted
		for _, delivery := range batch.deliveries {
			batch.entry.mailbox.deliverLocked(virtualDelivery{sender: delivery.sender, message: delivery.request.Message})
		}
	}
	v.pending -= len(batch.deliveries)
	var rejected []virtualRouteDelivery
	state := batch.state
	if len(state.deliveries) > 0 && status == virtual.DeliveryAccepted {
		v.checking = append(v.checking, &virtualRouteBatch{entry: batch.entry, state: state})
	} else {
		state.checking = false
		rejected, state.deliveries = state.deliveries, nil
		v.pending -= len(rejected)
	}
	v.mu.Unlock()
	for _, delivery := range batch.deliveries {
		v.replyRoute(delivery, status)
	}
	for _, delivery := range rejected {
		v.replyRoute(delivery, status)
	}
	if stopping != nil {
		v.system.Kill(stopping.ctx.ref, false, "virtual actor ownership lost")
	}
	v.mu.Lock()
	v.running--
	v.startLocked()
	v.finishOwnershipLocked()
	v.mu.Unlock()
}

func (v *virtualCoordinator) replyRoute(delivery virtualRouteDelivery, status virtual.DeliveryStatus) {
	m := delivery.request
	result := &virtual.DeliveryResult{Group: m.Group, Owner: m.Owner, Generation: m.Generation, Kind: m.Kind, Name: m.Name, Status: status}
	v.system.ResolveMailbox(delivery.reply).Enqueue(mailbox.NewEnvelop(false, v.system.ref, delivery.reply, result))
}
