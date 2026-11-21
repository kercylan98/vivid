package actor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func restartDiscoveryOwner(t *testing.T, n *raftRuntimeNetwork, previous *System, seen chan routedObservation, providers *atomic.Int32) (*System, *virtualOwnershipClient) {
	t.Helper()
	s := NewSystem(vivid.WithActorSystemRemoting(previous.options.RemotingBindAddress, previous.Ref().GetAddress()),
		vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnect(2, 20*time.Millisecond, 40*time.Millisecond, 2, false)),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			providers.Add(1)
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				virtualReply(ctx)
				if payload, ok := ctx.Message().(*Ref); ok {
					seen <- routedObservation{ctx.Sender().String(), payload.String()}
				}
			})
		})))
	owner, err := clusterstore.NewVirtualOwner(s.options.RemotingAdvertiseAddress)
	require.NoError(t, err)
	client, err := newVirtualOwnershipClient(s, clusterstore.RaftConfig{GroupID: "runtime", NodeID: 1, Voters: []uint64{1, 2, 3}}, n.refs, owner)
	require.NoError(t, err)
	require.NoError(t, s.virtualCoordinator.configureOwnership(client, time.Second))
	s.virtualCoordinator.options.IdleTimeout = time.Hour
	require.NoError(t, s.Start())
	t.Cleanup(func() { require.NoError(t, s.Stop(3*time.Second)) })
	return s, client
}

func waitDiscoveredOwner(t *testing.T, client *virtualOwnershipClient, owner clusterstore.VirtualOwner) {
	t.Helper()
	// 只重试只读发现以确认真实 TCP 重连；不重试未知业务投递或认领。
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		current, err := client.discover(ctx, owner.Address, "worker")
		return err == nil && current == owner
	}, 3*time.Second, 20*time.Millisecond)
}

func TestVirtualDiscovery_TCPSameAddressRestartAndOrphanRecord(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	seen := make(chan routedObservation, 8)
	previous, old := newRoutedOwner(t, n, 4, seen)
	var discoveries atomic.Int32
	caller := newRemoteControlSystemAt(t, n, 5, func(p probeRequest) bool {
		if _, ok := p.message.(*virtual.DiscoveryResult); ok {
			discoveries.Add(1)
		}
		return false
	})
	client := remoteOwnershipClient(t, caller, n)
	placements := map[string]string{"worker": old.owner.Address}
	router, err := newVirtualRouter(client, 8, 4, placements)
	require.NoError(t, err)
	delete(placements, "worker")
	key := virtualKey{"worker", "restart"}
	status, err := router.route(context.Background(), key, caller.Ref(), caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	receiveRetryResult(t, seen)
	first, exists, err := client.lookup(context.Background(), key)
	require.NoError(t, err)
	require.True(t, exists)
	for range 2 {
		status, err = router.route(context.Background(), key, caller.Ref(), caller.Ref())
		require.NoError(t, err)
		require.Equal(t, virtual.DeliveryAccepted, status)
		receiveRetryResult(t, seen)
	}
	require.EqualValues(t, 1, discoveries.Load(), "稳定缓存命中不重新发现启动身份")
	change, err := clusterstore.ClaimVirtualOwnership("worker", "orphan", old.owner, 0)
	require.NoError(t, err)
	orphan, _, err := n.changeOwnership(t, leader, change)
	require.NoError(t, err)
	require.NoError(t, previous.Stop(3*time.Second))
	var providers atomic.Int32
	_, next := restartDiscoveryOwner(t, n, previous, seen, &providers)
	require.NotEqual(t, old.owner.InstanceID, next.owner.InstanceID)
	waitDiscoveredOwner(t, client, next.owner)
	before := discoveries.Load()
	status, err = router.route(context.Background(), key, caller.Ref(), caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	receiveRetryResult(t, seen)
	current, exists, err := client.lookup(context.Background(), key)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, next.owner, current.Owner)
	require.Greater(t, current.Generation, first.Generation)
	require.EqualValues(t, before+1, discoveries.Load(), "旧缓存明确拒绝后才查询缺失记录的候选")
	before = discoveries.Load()
	_, err = router.route(context.Background(), virtualKey{"worker", "orphan"}, caller.Ref(), caller.Ref())
	require.ErrorIs(t, err, clusterstore.ErrVirtualOwnershipConflict)
	require.Equal(t, before, discoveries.Load(), "已有记录不能被候选发现替代")
	current, exists, err = client.lookup(context.Background(), virtualKey{"worker", "orphan"})
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, orphan, current)
	require.EqualValues(t, 1, providers.Load())
	require.NoError(t, client.release(context.Background(), orphan))
}

func TestVirtualDiscovery_LateOldIdentityCannotActivateOrReplaceCache(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	n.leader(t, 1, 2, 3)
	seen := make(chan routedObservation, 4)
	previous, old := newRoutedOwner(t, n, 4, seen)
	held := make(chan probeRequest, 1)
	var holding atomic.Bool
	holding.Store(true)
	caller := newRemoteControlSystemAt(t, n, 5, func(p probeRequest) bool {
		if _, ok := p.message.(*virtual.DiscoveryResult); ok && holding.CompareAndSwap(true, false) {
			held <- p
			return true
		}
		return false
	})
	client := remoteOwnershipClient(t, caller, n)
	router, err := newVirtualRouter(client, 8, 4, map[string]string{"worker": old.owner.Address})
	require.NoError(t, err)
	key := virtualKey{"worker", "late"}
	late := make(chan error, 1)
	go func() { _, err := router.route(context.Background(), key, caller.Ref(), caller.Ref()); late <- err }()
	response := receiveRetryResult(t, held)
	require.Equal(t, old.owner.InstanceID, response.message.(*virtual.DiscoveryResult).Owner)
	require.NoError(t, previous.Stop(3*time.Second))
	var providers atomic.Int32
	_, next := restartDiscoveryOwner(t, n, previous, seen, &providers)
	waitDiscoveredOwner(t, client, next.owner)
	status, err := router.route(context.Background(), key, caller.Ref(), caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	receiveRetryResult(t, seen)
	require.NoError(t, caller.remotingEnvelopHandler.(*controlEnvelopHandler).NetworkEnvelopHandler.HandleRemotingEnvelop(response.system, response.sender, response.receiver, response.message))
	require.ErrorIs(t, receiveRetryResult(t, late), clusterstore.ErrVirtualOwnershipConflict)
	cached, count := routerCache(router, key)
	require.Equal(t, next.owner, cached.Owner)
	require.Equal(t, 1, count)
	require.EqualValues(t, 1, providers.Load(), "迟到旧身份不能启动第二个实例或删除新的缓存观察")
	select {
	case <-seen:
		t.Fatal("旧发现请求不能继续发送业务")
	default:
	}
}

func TestVirtualDiscovery_DoesNotAuthorizeCreation(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	var providers atomic.Int32
	owner, oc := newRoutedOwner(t, n, 4, make(chan routedObservation, 4), func(s *System, _ *virtualOwnershipClient) {
		s.options.VirtualActorProviders["worker"] = vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			providers.Add(1)
			return vivid.ActorFN(virtualReply)
		})
	})
	caller := newRemoteControlSystemAt(t, n, 5, nil)
	client := remoteOwnershipClient(t, caller, n)
	current, err := client.discover(context.Background(), oc.owner.Address, "worker")
	require.NoError(t, err)
	require.Equal(t, oc.owner, current)
	require.Zero(t, providers.Load())
	_, exists, err := client.lookup(context.Background(), virtualKey{"worker", "one"})
	require.NoError(t, err)
	require.False(t, exists)
	_, err = client.discover(context.Background(), oc.owner.Address, "missing")
	require.ErrorIs(t, err, errVirtualActivationNotReady)
	other := remoteOwnershipClient(t, caller, n)
	other.control.group[0] ^= 1
	_, err = other.discover(context.Background(), oc.owner.Address, "worker")
	require.ErrorIs(t, err, virtual.ErrInvalidDiscovery)
	var isolated []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != leader {
			isolated = append(isolated, id)
		}
	}
	oc.control.leader.Store(leader)
	n.partition(isolated...)
	current, err = client.discover(context.Background(), oc.owner.Address, "worker")
	require.NoError(t, err, "发现不需要多数派，但也不提供创建资格")
	_, err = client.activate(context.Background(), virtualKey{"worker", "minority-discovery"}, current, 0)
	require.ErrorIs(t, err, errVirtualActivationUnknown)
	require.Zero(t, providers.Load())
	n.partition()
	owner.virtualCoordinator.close()
	_, err = client.discover(context.Background(), oc.owner.Address, "worker")
	require.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
}

func TestVirtualDiscovery_RepliesAreBoundToRequest(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	_, oc := newRoutedOwner(t, n, 4, make(chan routedObservation, 4))
	held := make(chan probeRequest, 1)
	caller := newRemoteControlSystemAt(t, n, 5, func(p probeRequest) bool {
		if _, ok := p.message.(*virtual.DiscoveryResult); ok {
			held <- p
			return true
		}
		return false
	})
	client := remoteOwnershipClient(t, caller, n)
	for _, change := range []func(*virtual.DiscoveryResult) any{
		func(m *virtual.DiscoveryResult) any { m.Group[0] ^= 1; return m },
		func(m *virtual.DiscoveryResult) any { m.Kind = "other"; return m },
		func(m *virtual.DiscoveryResult) any { m.Owner = [16]byte{}; return m },
		func(m *virtual.DiscoveryResult) any { m.Status = virtual.DiscoveryStopped; return m },
		func(*virtual.DiscoveryResult) any { return (*virtual.DiscoveryResult)(nil) },
	} {
		result := make(chan error, 1)
		go func() { _, err := client.discover(context.Background(), oc.owner.Address, "worker"); result <- err }()
		response := receiveRetryResult(t, held)
		message := change(response.message.(*virtual.DiscoveryResult))
		require.NoError(t, caller.remotingEnvelopHandler.(*controlEnvelopHandler).NetworkEnvelopHandler.HandleRemotingEnvelop(response.system, response.sender, response.receiver, message))
		require.ErrorIs(t, receiveRetryResult(t, result), virtual.ErrInvalidDiscovery)
	}
}

func TestVirtualDiscovery_RouterBudgetAndValidation(t *testing.T) {
	s, router, backend, value := newRouterTest(t, 4, 4)
	key := virtualKey{"worker", "one"}
	backend.lookupFn = func(context.Context, virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		return clusterstore.VirtualOwnership{}, false, nil
	}
	var discoveries, activations, sends int
	backend.deliverFn = func(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error) {
		sends++
		return virtual.DeliveryAccepted, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router.placements = map[string]string{"worker": value.Owner.Address}
	ready := &routerReadyTestBackend{routerTestBackend: backend,
		discoverFn: func(ctx context.Context, address, kind string) (clusterstore.VirtualOwner, error) {
			discoveries++
			_, ok := ctx.Deadline()
			require.True(t, ok)
			require.Equal(t, value.Owner.Address, address)
			require.Equal(t, key.kind, kind)
			cancel()
			return value.Owner, nil // 忽略取消的迟到成功不能继续激活。
		},
		activateFn: func(context.Context, virtualKey, clusterstore.VirtualOwner, uint64) (clusterstore.VirtualOwnership, error) {
			activations++
			return value, nil
		},
	}
	router.backend = ready
	_, err := router.route(ctx, key, s.Ref(), "payload")
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, discoveries)
	require.Zero(t, activations)
	require.Zero(t, sends)
	_, count := routerCache(router, key)
	require.Zero(t, count)
	ready.discoverFn = func(context.Context, string, string) (clusterstore.VirtualOwner, error) {
		other := value.Owner
		other.Address = "unexpected.example:9000"
		return other, nil
	}
	_, err = router.route(context.Background(), key, s.Ref(), "payload")
	require.ErrorIs(t, err, clusterstore.ErrInvalidVirtualOwnership)
	require.Zero(t, activations, "发现回执不能将请求重定向到其他地址")
	var deadline time.Time
	checkDeadline := func(ctx context.Context) {
		current, ok := ctx.Deadline()
		require.True(t, ok)
		if deadline.IsZero() {
			deadline = current
		}
		require.Equal(t, deadline, current, "归属查询、发现、激活和业务准入共享总预算")
	}
	backend.lookupFn = func(ctx context.Context, _ virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		checkDeadline(ctx)
		return clusterstore.VirtualOwnership{}, false, nil
	}
	ready.discoverFn = func(ctx context.Context, _, _ string) (clusterstore.VirtualOwner, error) {
		checkDeadline(ctx)
		return value.Owner, nil
	}
	ready.activateFn = func(ctx context.Context, _ virtualKey, target clusterstore.VirtualOwner, generation uint64) (clusterstore.VirtualOwnership, error) {
		checkDeadline(ctx)
		require.Equal(t, value.Owner, target)
		require.Zero(t, generation)
		return value, nil
	}
	backend.deliverFn = func(ctx context.Context, _ clusterstore.VirtualOwnership, _ vivid.ActorRef, _ vivid.Message) (virtual.DeliveryStatus, error) {
		checkDeadline(ctx)
		return virtual.DeliveryAccepted, nil
	}
	status, err := router.route(context.Background(), key, s.Ref(), "payload")
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	for kind, address := range map[string]string{"": value.Owner.Address, "worker": "bad/address", "too-long": string(make([]byte, 513))} {
		client := &virtualOwnershipClient{control: &raftControlClient{system: s}}
		_, err := newVirtualRouter(client, 4, 4, map[string]string{kind: address})
		require.Error(t, err)
	}
}

func TestVirtualDiscovery_LostReplyEndsWithinCallerBudget(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	n.leader(t, 1, 2, 3)
	var providers, discoveries atomic.Int32
	_, oc := newRoutedOwner(t, n, 4, make(chan routedObservation, 4), func(s *System, _ *virtualOwnershipClient) {
		s.options.VirtualActorProviders["worker"] = vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			providers.Add(1)
			return vivid.ActorFN(virtualReply)
		})
	})
	held := make(chan struct{}, 1)
	caller := newRemoteControlSystemAt(t, n, 5, func(p probeRequest) bool {
		if _, ok := p.message.(*virtual.DiscoveryResult); ok {
			discoveries.Add(1)
			held <- struct{}{}
			return true
		}
		return false
	})
	client := remoteOwnershipClient(t, caller, n)
	router, err := newVirtualRouter(client, 4, 1, map[string]string{"worker": oc.owner.Address})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := router.route(ctx, virtualKey{"worker", "lost-discovery"}, caller.Ref(), caller.Ref())
		result <- err
	}()
	receiveRetryResult(t, held)
	_, err = router.route(context.Background(), virtualKey{"worker", "other"}, caller.Ref(), caller.Ref())
	require.ErrorIs(t, err, errVirtualRouterBusy, "发现等待也占用同一个路由并发名额")
	require.ErrorIs(t, receiveRetryResult(t, result), context.DeadlineExceeded)
	require.EqualValues(t, 1, discoveries.Load())
	require.Zero(t, providers.Load())
	_, count := routerCache(router, virtualKey{"worker", "lost-discovery"})
	require.Zero(t, count)
	require.Empty(t, router.slots)
}
