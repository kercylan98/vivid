package actor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

type routingTestBackend struct {
	virtualOwnershipBackend
	group [32]byte
	owner clusterstore.VirtualOwner
}

func (b *routingTestBackend) routingIdentity() ([32]byte, clusterstore.VirtualOwner) {
	return b.group, b.owner
}

type routedObservation struct{ sender, payload string }

func newRoutedOwner(t *testing.T, n *raftRuntimeNetwork, id uint64, seen chan routedObservation, prepare ...func(*System, *virtualOwnershipClient)) (*System, *virtualOwnershipClient) {
	t.Helper()
	var client *virtualOwnershipClient
	system := newRemoteControlSystemAt(t, n, id, nil, func(s *System) {
		owner, err := clusterstore.NewVirtualOwner(s.options.RemotingAdvertiseAddress)
		require.NoError(t, err)
		client, err = newVirtualOwnershipClient(s, clusterstore.RaftConfig{GroupID: "runtime", NodeID: 1, Voters: []uint64{1, 2, 3}}, n.refs, owner)
		require.NoError(t, err)
		require.NoError(t, s.virtualCoordinator.configureOwnership(client, 300*time.Millisecond))
		s.virtualCoordinator.options.IdleTimeout = time.Hour
		s.options.VirtualActorProviders = map[string]vivid.VirtualActorProvider{"worker": vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				virtualReply(ctx)
				if payload, ok := ctx.Message().(*Ref); ok {
					seen <- routedObservation{ctx.Sender().String(), payload.String()}
					ctx.Reply(payload)
				}
			})
		})}
		for _, configure := range prepare {
			configure(s, client)
		}
	})
	return system, client
}

func TestVirtualRoute_TCPAdmissionMigrationAndLostReply(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	seen := make(chan routedObservation, 16)
	a, ac := newRoutedOwner(t, n, 4, seen)
	b, bc := newRoutedOwner(t, n, 5, seen)
	var lost atomic.Bool
	var receipts atomic.Int32
	caller := newRemoteControlSystemAt(t, n, 6, func(p probeRequest) bool {
		if _, ok := p.message.(*virtual.DeliveryResult); ok {
			receipts.Add(1)
			return lost.Load()
		}
		return false
	})
	client := remoteOwnershipClient(t, caller, n)
	replies := make(chan string, 16)
	sender, err := caller.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if m, ok := ctx.Message().(*Ref); ok {
			replies <- m.String()
		}
	}))
	require.NoError(t, err)
	key := virtualKey{"worker", "one"}
	require.NoError(t, a.Ask(a.VirtualRef(key.kind, key.name), 1).Wait())
	first, exists, err := client.lookup(context.Background(), key)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, ac.owner, first.Owner)
	m := waitVirtualMailbox(t, a, "one")
	m.Pause()
	status, err := client.deliver(context.Background(), first, sender, caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	select {
	case <-seen:
		t.Fatal("准入回执不能等待或代替业务执行")
	default:
	}
	m.Resume()
	require.Equal(t, routedObservation{sender.String(), caller.Ref().String()}, receiveRetryResult(t, seen))
	require.Equal(t, caller.Ref().String(), receiveRetryResult(t, replies), "业务应答回到原业务发送方")
	for _, change := range []func(*clusterstore.VirtualOwnership){
		func(v *clusterstore.VirtualOwnership) { v.Generation++ },
		func(v *clusterstore.VirtualOwnership) { v.Owner.InstanceID[0] ^= 1 },
		func(v *clusterstore.VirtualOwnership) { v.Name = "not-active" },
	} {
		bad := first
		change(&bad)
		status, err = client.deliver(context.Background(), bad, sender, caller.Ref())
		require.NoError(t, err)
		require.NotEqual(t, virtual.DeliveryAccepted, status)
	}
	wrong := *client
	other := remoteOwnershipClient(t, caller, n)
	other.control.group[0] ^= 1
	wrong.control = other.control
	status, err = wrong.deliver(context.Background(), first, sender, caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryInvalid, status)
	// 丢失的是已入队回执，不能换入口或自动重投。
	lost.Store(true)
	before := receipts.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	_, err = client.deliver(ctx, first, sender, caller.Ref())
	cancel()
	require.ErrorIs(t, err, errVirtualDeliveryUnknown)
	require.Equal(t, routedObservation{sender.String(), caller.Ref().String()}, receiveRetryResult(t, seen))
	require.Equal(t, caller.Ref().String(), receiveRetryResult(t, replies))
	require.Equal(t, before+1, receipts.Load())
	lost.Store(false)
	a.Kill(m.ctx.ref, false, "explicit migration")
	require.Eventually(t, func() bool {
		v := a.virtualCoordinator
		v.mu.Lock()
		defer v.mu.Unlock()
		return v.entries[key] == nil
	}, 2*time.Second, time.Millisecond)
	require.NoError(t, b.Ask(b.VirtualRef(key.kind, key.name), 1).Wait())
	next, exists, err := client.lookup(context.Background(), key)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, bc.owner, next.Owner)
	require.Greater(t, next.Generation, first.Generation)
	status, err = client.deliver(context.Background(), first, sender, caller.Ref())
	require.NoError(t, err)
	require.NotEqual(t, virtual.DeliveryAccepted, status)
	status, err = client.deliver(context.Background(), next, sender, caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	receiveRetryResult(t, seen)
	receiveRetryResult(t, replies)
	// 活跃旧实例不能只比较本地缓存：多数派上已改派给另一个启动身份。
	change, err := clusterstore.ClaimVirtualOwnership(key.kind, key.name, ac.owner, next.Generation)
	require.NoError(t, err)
	transferred, _, err := n.changeOwnership(t, leader, change)
	require.NoError(t, err)
	status, err = client.deliver(context.Background(), next, sender, caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryStale, status)
	require.Eventually(t, func() bool {
		v := b.virtualCoordinator
		v.mu.Lock()
		defer v.mu.Unlock()
		return v.entries[key] == nil
	}, 2*time.Second, time.Millisecond)
	current, exists := n.ownership(t, leader, key.kind, key.name)
	require.True(t, exists)
	require.Equal(t, transferred, current, "旧实例释放不能删除新归属")
	require.NoError(t, ac.release(context.Background(), transferred))
	select {
	case extra := <-seen:
		t.Fatalf("被拒绝或未知回执被重投: %v", extra)
	default:
	}
}

func newControlledRouteSystem(t *testing.T, limit int, validate func(context.Context, clusterstore.VirtualOwnership) error) (*System, *routingTestBackend) {
	t.Helper()
	s := NewSystem(vivid.WithActorSystemVirtualActorOptions(vivid.VirtualActorOptions{ActivationConcurrency: 1, PendingMessageLimit: limit, IdleTimeout: time.Hour}),
		vivid.WithActorSystemRemoting(admissionSystemAddress(t)),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor { return vivid.ActorFN(virtualReply) })))
	owner, err := clusterstore.NewVirtualOwner(s.options.RemotingAdvertiseAddress)
	if err != nil {
		t.Fatal(err)
	}
	b := &routingTestBackend{group: [32]byte{1}, owner: owner}
	b.virtualOwnershipBackend = &ownershipTestBackend{
		claimFn: func(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, error) {
			return clusterstore.VirtualOwnership{Kind: key.kind, Name: key.name, Owner: owner, Generation: 1}, nil
		},
		validateFn: validate,
		releaseFn:  func(context.Context, clusterstore.VirtualOwnership) error { return nil },
	}
	require.NoError(t, s.virtualCoordinator.configureOwnership(b, time.Second))
	require.NoError(t, s.Start())
	t.Cleanup(func() {
		if !s.requestsStopped() {
			require.NoError(t, s.Stop(3*time.Second))
		}
	})
	return s, b
}

func routeAsk(s *System, b *routingTestBackend, name string) vivid.Future[vivid.Message] {
	ref, _ := NewRef(s.Ref().GetAddress(), virtualDeliveryPath)
	return s.Ask(ref, &virtual.RoutedDelivery{Group: b.group, Owner: b.owner.InstanceID, Generation: 1, Kind: "worker", Name: name, Sender: s.Ref().String(), Message: struct{}{}}, 3*time.Second)
}

func routeResult(t *testing.T, f vivid.Future[vivid.Message], want virtual.DeliveryStatus) {
	t.Helper()
	result, err := f.Result()
	require.NoError(t, err)
	require.Equal(t, want, result.(*virtual.DeliveryResult).Status)
}

func TestVirtualRoute_BatchBudgetAndStop(t *testing.T) {
	var enabled atomic.Bool
	started, resume := make(chan struct{}, 4), make(chan struct{}, 4)
	var checks atomic.Int32
	s, b := newControlledRouteSystem(t, 3, func(ctx context.Context, _ clusterstore.VirtualOwnership) error {
		if !enabled.Load() {
			return nil
		}
		checks.Add(1)
		started <- struct{}{}
		select {
		case <-resume:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	require.NoError(t, s.Ask(s.VirtualRef("worker", "one"), 1).Wait())
	enabled.Store(true)
	first := routeAsk(s, b, "one")
	receiveRetryResult(t, started)
	second, third := routeAsk(s, b, "one"), routeAsk(s, b, "one")
	routeResult(t, routeAsk(s, b, "one"), virtual.DeliveryOverloaded)
	require.EqualValues(t, 1, checks.Load(), "同一身份最多一个在途确认")
	resume <- struct{}{}
	routeResult(t, first, virtual.DeliveryAccepted)
	receiveRetryResult(t, started)
	require.EqualValues(t, 2, checks.Load(), "确认开始后的新消息进入下一轮")
	resume <- struct{}{}
	routeResult(t, second, virtual.DeliveryAccepted)
	routeResult(t, third, virtual.DeliveryAccepted)
	fourth := routeAsk(s, b, "one")
	receiveRetryResult(t, started)
	fifth := routeAsk(s, b, "one")
	require.NoError(t, s.Stop(2*time.Second))
	routeResult(t, fourth, virtual.DeliveryStopped)
	routeResult(t, fifth, virtual.DeliveryStopped)
	v := s.virtualCoordinator
	v.mu.Lock()
	defer v.mu.Unlock()
	require.Zero(t, v.pending)
	require.Zero(t, v.running)
	require.Empty(t, v.checking)
}

func TestVirtualRoute_UnavailableDoesNotRevokeOwnership(t *testing.T) {
	var fail atomic.Bool
	s, b := newControlledRouteSystem(t, 3, func(context.Context, clusterstore.VirtualOwnership) error {
		if fail.Load() {
			return errors.New("temporary coordination failure")
		}
		return nil
	})
	require.NoError(t, s.Ask(s.VirtualRef("worker", "one"), 1).Wait())
	m := waitVirtualMailbox(t, s, "one")
	fail.Store(true)
	routeResult(t, routeAsk(s, b, "one"), virtual.DeliveryUnavailable)
	fail.Store(false)
	routeResult(t, routeAsk(s, b, "one"), virtual.DeliveryAccepted)
	require.Same(t, m, waitVirtualMailbox(t, s, "one"), "临时不可用不能当作接管或终止依据")
}

func TestVirtualRoute_TCPMinorityFailsClosedAndRecovers(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	seen := make(chan routedObservation, 4)
	owner, _ := newRoutedOwner(t, n, 4, seen)
	caller := newRemoteControlSystemAt(t, n, 5, nil)
	client := remoteOwnershipClient(t, caller, n)
	require.NoError(t, owner.Ask(owner.VirtualRef("worker", "one"), 1).Wait())
	first, exists, err := client.lookup(context.Background(), virtualKey{"worker", "one"})
	require.NoError(t, err)
	require.True(t, exists)
	var isolated []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != leader {
			isolated = append(isolated, id)
		}
	}
	n.partition(isolated...)
	status, err := client.deliver(context.Background(), first, caller.Ref(), caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryUnavailable, status, "仍能联系旧 Leader 不能代替多数派")
	select {
	case <-seen:
		t.Fatal("协调不可用时不得接纳")
	default:
	}
	n.partition()
	n.leader(t, 1, 2, 3)
	require.Eventually(t, func() bool {
		status, err = client.deliver(context.Background(), first, caller.Ref(), caller.Ref())
		require.NoError(t, err)
		require.Contains(t, []virtual.DeliveryStatus{virtual.DeliveryUnavailable, virtual.DeliveryAccepted}, status)
		return status == virtual.DeliveryAccepted
	}, 3*time.Second, 20*time.Millisecond, "恢复期间明确拒绝后，显式新请求可重新复核")
	receiveRetryResult(t, seen)
}

func TestVirtualRoute_SharesActivationBudgetAndReservesIngress(t *testing.T) {
	var enabled atomic.Bool
	started, resume := make(chan struct{}, 1), make(chan struct{})
	s, b := newControlledRouteSystem(t, 4, func(ctx context.Context, _ clusterstore.VirtualOwnership) error {
		if !enabled.Load() {
			return nil
		}
		started <- struct{}{}
		select {
		case <-resume:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	require.NoError(t, s.Ask(s.VirtualRef("worker", "one"), 1).Wait())
	_, err := s.ActorOf(vivid.ActorFN(virtualReply), vivid.WithActorName("@virtual-delivery"))
	require.Error(t, err, "内部入口不可被普通 Actor 占据")
	enabled.Store(true)
	first := routeAsk(s, b, "one")
	receiveRetryResult(t, started)
	pending := s.Ask(s.VirtualRef("worker", "two"), 1)
	v := s.virtualCoordinator
	v.mu.Lock()
	running, waiting, creating := v.running, len(v.waiting), v.entries[virtualKey{"worker", "two"}].creating
	v.mu.Unlock()
	require.Equal(t, 1, running)
	require.Equal(t, 1, waiting, "检查和激活共享并发上限")
	require.False(t, creating)
	enabled.Store(false)
	close(resume)
	routeResult(t, first, virtual.DeliveryAccepted)
	require.NoError(t, pending.Wait())
	routeResult(t, routeAsk(s, b, "absent"), virtual.DeliveryUnavailable)
	v.mu.Lock()
	absent := v.entries[virtualKey{"worker", "absent"}]
	v.mu.Unlock()
	require.Nil(t, absent, "远程投递不触发 Provider 或认领")
}

func TestVirtualRoute_LateConflictDoesNotCloseNewActivation(t *testing.T) {
	var block atomic.Bool
	started, resume := make(chan struct{}, 1), make(chan struct{})
	s, b := newControlledRouteSystem(t, 4, func(context.Context, clusterstore.VirtualOwnership) error {
		if block.CompareAndSwap(true, false) {
			started <- struct{}{}
			<-resume
			return clusterstore.ErrVirtualOwnershipConflict
		}
		return nil
	})
	var unblocked atomic.Bool
	defer func() {
		if unblocked.CompareAndSwap(false, true) {
			close(resume)
		}
	}()
	require.NoError(t, s.Ask(s.VirtualRef("worker", "one"), 1).Wait())
	old := waitVirtualMailbox(t, s, "one")
	v := s.virtualCoordinator
	v.mu.Lock()
	v.options.ActivationConcurrency = 2
	v.mu.Unlock()
	backend := b.virtualOwnershipBackend.(*ownershipTestBackend)
	backend.claimFn = func(_ context.Context, key virtualKey) (clusterstore.VirtualOwnership, error) {
		return clusterstore.VirtualOwnership{Kind: key.kind, Name: key.name, Owner: b.owner, Generation: 2}, nil
	}
	block.Store(true)
	late := routeAsk(s, b, "one")
	receiveRetryResult(t, started)
	s.Kill(old.ctx.ref, false, "late route confirmation")
	require.Eventually(t, func() bool {
		v.mu.Lock()
		defer v.mu.Unlock()
		return v.entries[virtualKey{"worker", "one"}] == nil
	}, time.Second, time.Millisecond)
	require.NoError(t, s.Ask(s.VirtualRef("worker", "one"), 1).Wait())
	next := waitVirtualMailbox(t, s, "one")
	require.NotSame(t, old, next)
	unblocked.Store(true)
	close(resume)
	routeResult(t, late, virtual.DeliveryStale)
	require.NoError(t, s.Ask(s.VirtualRef("worker", "one"), 1).Wait())
	require.Same(t, next, waitVirtualMailbox(t, s, "one"), "迟到旧检查不能关闭同名新代")
}
