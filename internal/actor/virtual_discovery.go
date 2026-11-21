package actor

import (
	"context"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/virtual"
)

// discover 只取得联系线索，不缓存启动身份，也不因失败换入口或发起激活。
func (c *virtualOwnershipClient) discover(ctx context.Context, address, kind string) (clusterstore.VirtualOwner, error) {
	request := &virtual.DiscoveryRequest{Group: c.control.group, Kind: kind}
	ref, err := NewRef(address, virtualDeliveryPath)
	if err != nil || !request.Valid() {
		return clusterstore.VirtualOwner{}, virtual.ErrInvalidDiscovery
	}
	if err := ctx.Err(); err != nil {
		return clusterstore.VirtualOwner{}, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	future := c.control.system.Ask(ref, request, 0)
	stop := context.AfterFunc(ctx, func() { future.Close(ctx.Err()) })
	defer stop()
	result, err := future.Result()
	if err != nil {
		return clusterstore.VirtualOwner{}, err
	}
	if err := ctx.Err(); err != nil {
		return clusterstore.VirtualOwner{}, err
	}
	response, ok := result.(*virtual.DiscoveryResult)
	if !ok || !response.Valid() || response.Group != request.Group || response.Kind != request.Kind {
		return clusterstore.VirtualOwner{}, virtual.ErrInvalidDiscovery
	}
	switch response.Status {
	case virtual.DiscoveryAvailable:
		owner := clusterstore.VirtualOwner{Address: ref.GetAddress(), InstanceID: response.Owner}
		if _, err := clusterstore.ClaimVirtualOwnership(kind, "validate", owner, 0); err != nil {
			return clusterstore.VirtualOwner{}, err
		}
		return owner, nil
	case virtual.DiscoveryStopped:
		return clusterstore.VirtualOwner{}, vivid.ErrorActorSystemStopped
	case virtual.DiscoveryInvalid:
		return clusterstore.VirtualOwner{}, virtual.ErrInvalidDiscovery
	default:
		return clusterstore.VirtualOwner{}, errVirtualActivationNotReady
	}
}

func (m *virtualIngressMailbox) discover(envelope vivid.Envelop, request *virtual.DiscoveryRequest) {
	if !request.Valid() || envelope.Sender() == nil {
		return
	}
	v := m.coordinator
	result := &virtual.DiscoveryResult{Group: request.Group, Kind: request.Kind, Status: virtual.DiscoveryInvalid}
	if !envelope.System() && envelope.Receiver() != nil && envelope.Receiver().GetAddress() == v.system.Ref().GetAddress() && envelope.Receiver().GetPath() == virtualDeliveryPath {
		v.mu.Lock()
		v.system.statusLock.Lock()
		started := v.system.status == start
		v.system.statusLock.Unlock()
		switch {
		case v.closed:
			result.Status = virtual.DiscoveryStopped
		case v.gate == nil || v.gate.group != request.Group || v.gate.owner.Address != v.system.Ref().GetAddress():
			result.Status = virtual.DiscoveryInvalid
		case !started || v.system.options.VirtualActorProviders[request.Kind] == nil:
			result.Status = virtual.DiscoveryUnavailable
		default:
			result.Status, result.Owner = virtual.DiscoveryAvailable, v.gate.owner.InstanceID
		}
		v.mu.Unlock()
	}
	// 发现不进入激活等待，不查询 Raft、不调用 Provider；回执发出后入口仍可能停止或重启。
	v.system.ResolveMailbox(envelope.Sender()).Enqueue(mailbox.NewEnvelop(false, v.system.ref, envelope.Sender(), result))
}
