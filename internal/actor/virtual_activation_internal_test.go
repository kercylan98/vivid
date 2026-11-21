package actor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/stretchr/testify/require"
)

type routerReadyTestBackend struct {
	*routerTestBackend
	activateFn func(context.Context, virtualKey, clusterstore.VirtualOwner, uint64) (clusterstore.VirtualOwnership, error)
}

func (b *routerReadyTestBackend) activate(ctx context.Context, key virtualKey, target clusterstore.VirtualOwner, generation uint64) (clusterstore.VirtualOwnership, error) {
	return b.activateFn(ctx, key, target, generation)
}

func TestVirtualActivation_RouterRetriesSameGenerationOnlyAfterReady(t *testing.T) {
	s, r, backend, value := newRouterTest(t, 4, 4)
	var confirmations, sends int
	r.backend = &routerReadyTestBackend{routerTestBackend: backend, activateFn: func(ctx context.Context, key virtualKey, target clusterstore.VirtualOwner, generation uint64) (clusterstore.VirtualOwnership, error) {
		confirmations++
		return value, nil
	}}
	backend.deliverFn = func(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error) {
		sends++
		if sends == 1 {
			return virtual.DeliveryUnavailable, nil
		}
		return virtual.DeliveryAccepted, nil
	}
	status, err := r.route(context.Background(), virtualKey{"worker", "one"}, s.Ref(), "payload")
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	require.Equal(t, 2, sends)
	require.Equal(t, 2, confirmations, "明确拒绝后的新 Ready 确认允许同代次再发送一次")
}

func TestVirtualActivation_TCPConcurrentColdRouteAndPassivation(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	n.leader(t, 1, 2, 3)
	seen := make(chan routedObservation, 64)
	var providers atomic.Int32
	owner, ownerClient := newRoutedOwner(t, n, 4, seen, func(s *System, _ *virtualOwnershipClient) {
		provider := s.options.VirtualActorProviders["worker"]
		s.options.VirtualActorProviders["worker"] = vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			providers.Add(1)
			return provider.Provide(kind, name)
		})
	})
	caller := newRemoteControlSystemAt(t, n, 5, nil)
	client := remoteOwnershipClient(t, caller, n)
	placements := map[string]clusterstore.VirtualOwner{"worker": ownerClient.owner}
	router, err := newVirtualRouter(client, 16, 32, placements)
	require.NoError(t, err)
	delete(placements, "worker") // 外部修改配置不能改变已安装的候选。
	key := virtualKey{"worker", "cold"}
	replies := make(chan bool, 32)
	for range 16 {
		go func() {
			status, err := router.route(context.Background(), key, caller.Ref(), caller.Ref())
			replies <- err == nil && status == virtual.DeliveryAccepted
		}()
	}
	for range 16 {
		require.True(t, receiveRetryResult(t, replies))
		require.Equal(t, caller.Ref().String(), receiveRetryResult(t, seen).sender)
	}
	require.EqualValues(t, 1, providers.Load(), "并发远程首次激活只调用一次 Provider")
	first, exists, err := client.lookup(context.Background(), key)
	require.NoError(t, err)
	require.True(t, exists)
	expireVirtualMailbox(waitVirtualMailbox(t, owner, key.name))
	require.Eventually(t, func() bool {
		v := owner.virtualCoordinator
		v.mu.Lock()
		defer v.mu.Unlock()
		return v.entries[key] == nil
	}, 2*time.Second, time.Millisecond)
	status, err := router.route(context.Background(), key, caller.Ref(), caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status, "旧缓存明确拒绝后可按候选重新激活")
	receiveRetryResult(t, seen)
	second, exists, err := client.lookup(context.Background(), key)
	require.NoError(t, err)
	require.True(t, exists)
	require.Greater(t, second.Generation, first.Generation)
	require.EqualValues(t, 2, providers.Load())
}

func TestVirtualActivation_ClaimIsNotReadyAndStopEndsWaiting(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "stop"}[stop], func(t *testing.T) {
			n := newRaftRuntimeNetwork(t, false, nil)
			n.leader(t, 1, 2, 3)
			seen := make(chan routedObservation, 4)
			entered, release := make(chan struct{}, 1), make(chan struct{})
			var released atomic.Bool
			unblock := func() {
				if released.CompareAndSwap(false, true) {
					close(release)
				}
			}
			defer unblock()
			owner, oc := newRoutedOwner(t, n, 4, seen, func(s *System, _ *virtualOwnershipClient) {
				provider := s.options.VirtualActorProviders["worker"]
				s.options.VirtualActorProviders["worker"] = vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
					return vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error {
						entered <- struct{}{}
						<-release
						return nil
					}, provider.Provide(kind, name))
				})
			})
			caller := newRemoteControlSystemAt(t, n, 5, nil)
			client := remoteOwnershipClient(t, caller, n)
			key := virtualKey{"worker", "blocked"}
			result := make(chan error, 1)
			go func() { _, err := client.activate(context.Background(), key, oc.owner, 0); result <- err }()
			receiveRetryResult(t, entered)
			record, exists, err := client.lookup(context.Background(), key)
			require.NoError(t, err)
			require.True(t, exists, "归属已提交，但预启动尚未返回")
			select {
			case <-result:
				t.Fatal("取得归属不能提前报告 Ready")
			default:
			}
			status, err := client.deliver(context.Background(), record, caller.Ref(), caller.Ref())
			require.NoError(t, err)
			require.Equal(t, virtual.DeliveryUnavailable, status)
			if stop {
				require.ErrorIs(t, owner.Stop(20*time.Millisecond), vivid.ErrorActorSystemStopFailed)
				require.ErrorIs(t, receiveRetryResult(t, result), vivid.ErrorActorSystemStopped)
				unblock()
				require.NoError(t, owner.Stop(2*time.Second))
				_, exists, err = client.lookup(context.Background(), key)
				require.NoError(t, err)
				require.False(t, exists, "停止完成前须通过仍存活的控制入口释放已取得的归属")
			} else {
				unblock()
				require.NoError(t, receiveRetryResult(t, result))
				status, err = client.deliver(context.Background(), record, caller.Ref(), caller.Ref())
				require.NoError(t, err)
				require.Equal(t, virtual.DeliveryAccepted, status)
				receiveRetryResult(t, seen)
			}
			v := owner.virtualCoordinator
			require.Eventually(t, func() bool { v.mu.Lock(); defer v.mu.Unlock(); return v.pending == 0 && v.running == 0 }, time.Second, time.Millisecond)
		})
	}
}

func TestVirtualActivation_LostReadyDoesNotSendBusiness(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	n.leader(t, 1, 2, 3)
	seen := make(chan routedObservation, 4)
	var providers atomic.Int32
	_, oc := newRoutedOwner(t, n, 4, seen, func(s *System, _ *virtualOwnershipClient) {
		provider := s.options.VirtualActorProviders["worker"]
		s.options.VirtualActorProviders["worker"] = vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor { providers.Add(1); return provider.Provide(kind, name) })
	})
	var dropped atomic.Bool
	var receipts atomic.Int32
	dropped.Store(true)
	caller := newRemoteControlSystemAt(t, n, 5, func(p probeRequest) bool {
		if _, ok := p.message.(*virtual.ActivationResult); ok {
			receipts.Add(1)
			return dropped.Load()
		}
		return false
	})
	client := remoteOwnershipClient(t, caller, n)
	router, err := newVirtualRouter(client, 16, 16, map[string]clusterstore.VirtualOwner{"worker": oc.owner})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, err = router.route(ctx, virtualKey{"worker", "lost"}, caller.Ref(), caller.Ref())
	cancel()
	require.ErrorIs(t, err, errVirtualActivationUnknown)
	require.EqualValues(t, 1, receipts.Load())
	require.EqualValues(t, 1, providers.Load())
	select {
	case <-seen:
		t.Fatal("Ready 回执未知时不能开始投递业务")
	default:
	}
	dropped.Store(false)
	status, err := router.route(context.Background(), virtualKey{"worker", "lost"}, caller.Ref(), caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	receiveRetryResult(t, seen)
	require.EqualValues(t, 1, providers.Load(), "显式新调用确认现有实例，不重新认领")
}

func TestVirtualActivation_RejectsWrongIdentityAndNeverAdoptsExistingRecord(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	var providers atomic.Int32
	_, oc := newRoutedOwner(t, n, 4, make(chan routedObservation, 4), func(s *System, _ *virtualOwnershipClient) {
		provider := s.options.VirtualActorProviders["worker"]
		s.options.VirtualActorProviders["worker"] = vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor { providers.Add(1); return provider.Provide(kind, name) })
	})
	caller := newRemoteControlSystemAt(t, n, 5, nil)
	client := remoteOwnershipClient(t, caller, n)
	key := virtualKey{"worker", "reserved"}
	wrong := oc.owner
	wrong.InstanceID[0] ^= 1
	_, err := client.activate(context.Background(), key, wrong, 0)
	require.ErrorIs(t, err, clusterstore.ErrVirtualOwnershipConflict)
	other := remoteOwnershipClient(t, caller, n)
	other.control.group[0] ^= 1
	_, err = other.activate(context.Background(), key, oc.owner, 0)
	require.ErrorIs(t, err, virtual.ErrInvalidActivation)
	_, err = client.activate(context.Background(), key, oc.owner, 1)
	require.ErrorIs(t, err, errVirtualActivationNotReady)
	require.Zero(t, providers.Load())
	change, err := clusterstore.ClaimVirtualOwnership(key.kind, key.name, oc.owner, 0)
	require.NoError(t, err)
	reserved, _, err := n.changeOwnership(t, leader, change)
	require.NoError(t, err)
	_, err = client.activate(context.Background(), key, oc.owner, reserved.Generation)
	require.ErrorIs(t, err, errVirtualActivationNotReady, "已有记录不授权恢复没有本地激活条目的实例")
	_, err = client.activate(context.Background(), key, oc.owner, 0)
	require.ErrorIs(t, err, clusterstore.ErrVirtualOwnershipConflict, "零代次仅可 CAS 不存在的记录")
	require.Zero(t, providers.Load())
	require.NoError(t, oc.release(context.Background(), reserved))
}

func TestVirtualActivation_FailedPrelaunchReleasesBeforeNewAttempt(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	n.leader(t, 1, 2, 3)
	var providers atomic.Int32
	owner, oc := newRoutedOwner(t, n, 4, make(chan routedObservation, 4), func(s *System, _ *virtualOwnershipClient) {
		provider := s.options.VirtualActorProviders["worker"]
		s.options.VirtualActorProviders["worker"] = vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			if providers.Add(1) == 1 {
				return vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error { return context.Canceled }, provider.Provide(kind, name))
			}
			return provider.Provide(kind, name)
		})
	})
	caller := newRemoteControlSystemAt(t, n, 5, nil)
	client := remoteOwnershipClient(t, caller, n)
	key := virtualKey{"worker", "failed"}
	_, err := client.activate(context.Background(), key, oc.owner, 0)
	require.ErrorIs(t, err, vivid.ErrorActorSpawnFailed)
	require.Eventually(t, func() bool {
		v := owner.virtualCoordinator
		v.mu.Lock()
		defer v.mu.Unlock()
		return v.entries[key] == nil
	}, 2*time.Second, time.Millisecond)
	record, err := client.activate(context.Background(), key, oc.owner, 0)
	require.NoError(t, err)
	require.NotZero(t, record.Generation)
	require.EqualValues(t, 2, providers.Load())
}

func TestVirtualActivation_MinorityClaimIsUnknownWithoutProvider(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	var providers atomic.Int32
	_, oc := newRoutedOwner(t, n, 4, make(chan routedObservation, 4), func(s *System, _ *virtualOwnershipClient) {
		s.options.VirtualActorProviders["worker"] = vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor { providers.Add(1); return vivid.ActorFN(virtualReply) })
	})
	caller := newRemoteControlSystemAt(t, n, 5, nil)
	client := remoteOwnershipClient(t, caller, n)
	var isolated []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != leader {
			isolated = append(isolated, id)
		}
	}
	oc.control.leader.Store(leader) // 直接联系仍可达的旧 Leader，确保实际发送认领后才等待多数派。
	n.partition(isolated...)
	_, err := client.activate(context.Background(), virtualKey{"worker", "minority"}, oc.owner, 0)
	require.ErrorIs(t, err, errVirtualActivationUnknown)
	require.Zero(t, providers.Load(), "联系旧 Leader 不授权创建业务实例")
	n.partition()
}

func TestVirtualActivation_ReadyRequiresFreshMajorityAndWaitersAreBounded(t *testing.T) {
	var mode atomic.Int32
	entered, release := make(chan struct{}, 4), make(chan struct{}, 4)
	s, backend := newControlledRouteSystem(t, 2, func(ctx context.Context, _ clusterstore.VirtualOwnership) error {
		switch mode.Load() {
		case 1:
			return context.DeadlineExceeded
		case 2:
			entered <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
	key := virtualKey{"worker", "one"}
	require.NoError(t, s.Ask(s.VirtualRef(key.kind, key.name), 1).Wait())
	client, err := newVirtualOwnershipClient(s, clusterstore.RaftConfig{GroupID: "ready-test", NodeID: 1, Voters: []uint64{1}}, map[uint64]vivid.ActorRef{1: s.Ref()}, backend.owner)
	require.NoError(t, err)
	client.control.group = backend.group
	mode.Store(1)
	_, err = client.activate(context.Background(), key, backend.owner, 1)
	require.ErrorIs(t, err, errVirtualActivationNotReady, "本地缓存代次匹配不能代替本次多数派确认")
	mode.Store(2)
	first := make(chan error, 1)
	go func() { _, err := client.activate(context.Background(), key, backend.owner, 1); first <- err }()
	receiveRetryResult(t, entered)
	second := make(chan error, 1)
	go func() { _, err := client.activate(context.Background(), key, backend.owner, 1); second <- err }()
	v := s.virtualCoordinator
	require.Eventually(t, func() bool { v.mu.Lock(); defer v.mu.Unlock(); return v.pending == 2 }, time.Second, time.Millisecond)
	_, err = client.activate(context.Background(), key, backend.owner, 1)
	require.ErrorIs(t, err, vivid.ErrorVirtualPendingMessagesFull)
	release <- struct{}{}
	require.NoError(t, receiveRetryResult(t, first))
	receiveRetryResult(t, entered)
	release <- struct{}{}
	require.NoError(t, receiveRetryResult(t, second))
}

func TestVirtualActivation_PrelaunchLossCannotReportReady(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var released atomic.Bool
	defer func() {
		if released.CompareAndSwap(false, true) {
			close(release)
		}
	}()
	owner, oc := newRoutedOwner(t, n, 4, make(chan routedObservation, 4), func(s *System, _ *virtualOwnershipClient) {
		provider := s.options.VirtualActorProviders["worker"]
		s.options.VirtualActorProviders["worker"] = vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			return vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error { entered <- struct{}{}; <-release; return nil }, provider.Provide(kind, name))
		})
	})
	caller := newRemoteControlSystemAt(t, n, 5, nil)
	client := remoteOwnershipClient(t, caller, n)
	key := virtualKey{"worker", "lost-during-init"}
	result := make(chan error, 1)
	go func() { _, err := client.activate(context.Background(), key, oc.owner, 0); result <- err }()
	receiveRetryResult(t, entered)
	first, exists, err := client.lookup(context.Background(), key)
	require.NoError(t, err)
	require.True(t, exists)
	nextOwner, err := clusterstore.NewVirtualOwner("new-owner.example:9000")
	require.NoError(t, err)
	change, err := clusterstore.ClaimVirtualOwnership(key.kind, key.name, nextOwner, first.Generation)
	require.NoError(t, err)
	next, _, err := n.changeOwnership(t, leader, change)
	require.NoError(t, err)
	released.Store(true)
	close(release)
	require.ErrorIs(t, receiveRetryResult(t, result), clusterstore.ErrVirtualOwnershipConflict)
	require.Eventually(t, func() bool {
		v := owner.virtualCoordinator
		v.mu.Lock()
		defer v.mu.Unlock()
		return v.entries[key] == nil
	}, time.Second, time.Millisecond)
	current, exists := n.ownership(t, leader, key.kind, key.name)
	require.True(t, exists)
	require.Equal(t, next, current)
}

func TestVirtualActivation_ExpiredConfirmationCannotReportReady(t *testing.T) {
	var block atomic.Bool
	entered, release := make(chan struct{}, 1), make(chan struct{}, 1)
	s, backend := newControlledRouteSystem(t, 2, func(ctx context.Context, _ clusterstore.VirtualOwnership) error {
		if block.Load() {
			entered <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
	defer close(release)
	require.NoError(t, s.Ask(s.VirtualRef("worker", "one"), 1).Wait())
	block.Store(true)
	ref, err := NewRef(s.Ref().GetAddress(), virtualDeliveryPath)
	require.NoError(t, err)
	request := &virtual.ActivationRequest{Group: backend.group, Owner: backend.owner.InstanceID, Generation: 1, Kind: "worker", Name: "one", Timeout: 10 * time.Millisecond}
	result := s.Ask(ref, request, time.Second)
	receiveRetryResult(t, entered)
	// 复核已开始并跨过请求期限，即使多数派确认成功也不能再报告就绪。
	<-time.After(2 * request.Timeout)
	release <- struct{}{}
	message, err := result.Result()
	require.NoError(t, err)
	response := message.(*virtual.ActivationResult)
	require.Equal(t, virtual.ActivationUnavailable, response.Status)
	require.Zero(t, response.CurrentGeneration)
	require.True(t, response.Valid())
}
