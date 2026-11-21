package actor

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

type routerTestBackend struct {
	lookupFn  func(context.Context, virtualKey) (clusterstore.VirtualOwnership, bool, error)
	deliverFn func(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error)
}

func (b *routerTestBackend) lookup(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, bool, error) {
	return b.lookupFn(ctx, key)
}

func (b *routerTestBackend) deliver(ctx context.Context, value clusterstore.VirtualOwnership, sender vivid.ActorRef, message vivid.Message) (virtual.DeliveryStatus, error) {
	return b.deliverFn(ctx, value, sender, message)
}

func newRouterTest(t testing.TB, capacity, concurrency int) (*System, *virtualRouter, *routerTestBackend, clusterstore.VirtualOwnership) {
	t.Helper()
	s := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))))
	require.NoError(t, s.Start())
	t.Cleanup(func() {
		if !s.requestsStopped() {
			require.NoError(t, s.Stop(time.Second))
		}
	})
	owner, err := clusterstore.NewVirtualOwner("owner.example:9000")
	require.NoError(t, err)
	c, err := newVirtualOwnershipClient(s, clusterstore.RaftConfig{GroupID: "router-test", NodeID: 1, Voters: []uint64{1}}, map[uint64]vivid.ActorRef{1: s.Ref()}, owner)
	require.NoError(t, err)
	r, err := newVirtualRouter(c, capacity, concurrency)
	require.NoError(t, err)
	value := clusterstore.VirtualOwnership{Kind: "worker", Name: "one", Owner: owner, Generation: 1}
	b := &routerTestBackend{
		lookupFn: func(context.Context, virtualKey) (clusterstore.VirtualOwnership, bool, error) {
			return value, true, nil
		},
		deliverFn: func(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error) {
			return virtual.DeliveryAccepted, nil
		},
	}
	r.backend = b
	return s, r, b, value
}

func routerCache(r *virtualRouter, key virtualKey) (clusterstore.VirtualOwnership, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if hint := r.cache[key]; hint != nil {
		return hint.ownership, len(r.cache)
	}
	return clusterstore.VirtualOwnership{}, len(r.cache)
}

func TestVirtualRouter_RelocationPolicy(t *testing.T) {
	for _, first := range []virtual.DeliveryStatus{virtual.DeliveryAccepted, virtual.DeliveryStale, virtual.DeliveryUnavailable, virtual.DeliveryStopped, virtual.DeliveryOverloaded, virtual.DeliveryInvalid} {
		for _, changed := range []bool{false, true} {
			t.Run(time.Duration(first).String()+"/"+map[bool]string{false: "same", true: "new"}[changed], func(t *testing.T) {
				s, r, b, old := newRouterTest(t, 2, 2)
				var reads, sends int
				var deadline time.Time
				b.lookupFn = func(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, bool, error) {
					reads++
					d, ok := ctx.Deadline()
					require.True(t, ok)
					if reads == 1 {
						deadline = d
					}
					require.Equal(t, deadline, d, "所有阶段共用一个期限")
					value := old
					if reads > 1 && changed {
						value.Generation++
						value.Owner.InstanceID[0] ^= 1 // 同地址新启动也须按新代次重定位
					}
					return value, true, nil
				}
				b.deliverFn = func(ctx context.Context, value clusterstore.VirtualOwnership, sender vivid.ActorRef, message vivid.Message) (virtual.DeliveryStatus, error) {
					sends++
					d, _ := ctx.Deadline()
					require.Equal(t, deadline, d)
					require.Same(t, s.Ref(), sender)
					require.Equal(t, "payload", message)
					if sends == 1 {
						return first, nil
					}
					require.Greater(t, value.Generation, old.Generation)
					return virtual.DeliveryAccepted, nil
				}
				status, err := r.route(context.Background(), virtualKey{"worker", "one"}, s.Ref(), "payload")
				require.NoError(t, err)
				wantReads, wantSends := 1, 1
				want := first
				if relocatableDelivery(first) {
					wantReads++
					if changed {
						wantSends++
						want = virtual.DeliveryAccepted
					}
				}
				require.Equal(t, want, status)
				require.Equal(t, wantReads, reads)
				require.Equal(t, wantSends, sends)
				cached, size := routerCache(r, virtualKey{"worker", "one"})
				if want == virtual.DeliveryAccepted || want == virtual.DeliveryOverloaded {
					require.Equal(t, 1, size, "接纳或容量拒绝不失效位置线索")
					require.NotZero(t, cached.Generation)
				} else {
					require.Zero(t, size)
				}
			})
		}
	}
}

func TestVirtualRouter_UnknownIsNotRetriedAndInvalidatesHint(t *testing.T) {
	s, r, b, value := newRouterTest(t, 2, 2)
	var reads, sends int
	b.lookupFn = func(context.Context, virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		reads++
		value.Generation++
		return value, true, nil
	}
	b.deliverFn = func(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error) {
		sends++
		if sends == 1 {
			return 0, errors.Join(errVirtualDeliveryUnknown, context.DeadlineExceeded)
		}
		return virtual.DeliveryAccepted, nil
	}
	key := virtualKey{"worker", "one"}
	_, err := r.route(context.Background(), key, s.Ref(), "payload")
	require.ErrorIs(t, err, errVirtualDeliveryUnknown)
	require.Equal(t, 1, reads)
	require.Equal(t, 1, sends)
	_, size := routerCache(r, key)
	require.Zero(t, size)
	status, err := r.route(context.Background(), key, s.Ref(), "new-message")
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	require.Equal(t, 2, reads, "后续新调用重新解析，不能重发已结束的旧消息")
	require.Equal(t, 2, sends)
}

func TestVirtualRouter_SecondRejectionEndsRelocation(t *testing.T) {
	s, r, b, value := newRouterTest(t, 2, 2)
	var reads, sends int
	b.lookupFn = func(context.Context, virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		reads++
		value.Generation++
		return value, true, nil
	}
	b.deliverFn = func(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error) {
		sends++
		return virtual.DeliveryStale, nil
	}
	status, err := r.route(context.Background(), virtualKey{"worker", "one"}, s.Ref(), "payload")
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryStale, status)
	require.Equal(t, 2, reads)
	require.Equal(t, 2, sends, "持续迁移也只重定位一次")
}

func TestVirtualRouter_NoNegativeCacheAndBoundedLRU(t *testing.T) {
	s, r, b, value := newRouterTest(t, 2, 2)
	absent := true
	reads := make(map[virtualKey]int)
	b.lookupFn = func(_ context.Context, key virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		reads[key]++
		v := value
		v.Name = key.name
		return v, !absent, nil
	}
	one, two, three := virtualKey{"worker", "one"}, virtualKey{"worker", "two"}, virtualKey{"worker", "three"}
	_, err := r.route(context.Background(), one, s.Ref(), "payload")
	require.ErrorIs(t, err, errVirtualRouteNotFound)
	absent = false
	for _, key := range []virtualKey{one, two, one, three} {
		status, err := r.route(context.Background(), key, s.Ref(), "payload")
		require.NoError(t, err)
		require.Equal(t, virtual.DeliveryAccepted, status)
	}
	_, size := routerCache(r, one)
	require.Equal(t, 2, size)
	cached, _ := routerCache(r, two)
	require.Zero(t, cached.Generation, "淘汰最久未用项")
	require.Equal(t, 2, reads[one], "不存在结果不被缓存，稳定命中不重复查询")
	_, err = r.route(context.Background(), two, s.Ref(), "payload")
	require.NoError(t, err)
	require.Equal(t, 2, reads[two])
}

func TestVirtualRouter_LateLookupAndRejectionPreserveNewHint(t *testing.T) {
	for _, stage := range []string{"lookup", "delivery"} {
		t.Run(stage, func(t *testing.T) {
			s, r, b, old := newRouterTest(t, 2, 3)
			next := old
			next.Generation++
			entered, release := make(chan struct{}, 1), make(chan struct{})
			var released atomic.Bool
			defer func() {
				if released.CompareAndSwap(false, true) {
					close(release)
				}
			}()
			var reads, sends atomic.Int32
			b.lookupFn = func(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, bool, error) {
				if reads.Add(1) == 1 {
					if stage == "lookup" {
						entered <- struct{}{}
						<-release
					}
					return old, true, nil
				}
				return next, true, nil
			}
			b.deliverFn = func(ctx context.Context, value clusterstore.VirtualOwnership, sender vivid.ActorRef, message vivid.Message) (virtual.DeliveryStatus, error) {
				if value.Generation == old.Generation {
					if stage == "delivery" && sends.Add(1) == 1 {
						entered <- struct{}{}
						<-release
					}
					return virtual.DeliveryStale, nil
				}
				return virtual.DeliveryAccepted, nil
			}
			key := virtualKey{"worker", "one"}
			result := make(chan error, 1)
			go func() {
				status, err := r.route(context.Background(), key, s.Ref(), "old")
				if err == nil && status != virtual.DeliveryAccepted {
					err = errors.New("late request was not accepted")
				}
				result <- err
			}()
			receiveRetryResult(t, entered)
			status, err := r.route(context.Background(), key, s.Ref(), "new")
			require.NoError(t, err)
			require.Equal(t, virtual.DeliveryAccepted, status)
			cached, _ := routerCache(r, key)
			require.Equal(t, next, cached)
			released.Store(true)
			close(release)
			require.NoError(t, receiveRetryResult(t, result))
			cached, _ = routerCache(r, key)
			require.Equal(t, next, cached, "旧观察不能覆盖或失效新缓存")
			require.EqualValues(t, 2, reads.Load())
		})
	}
}

func TestVirtualRouter_BudgetCancellationAndStop(t *testing.T) {
	s, r, b, value := newRouterTest(t, 2, 1)
	entered := make(chan struct{}, 1)
	b.lookupFn = func(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return value, false, ctx.Err()
	}
	result := make(chan error, 1)
	go func() {
		_, err := r.route(context.Background(), virtualKey{"worker", "one"}, s.Ref(), "payload")
		result <- err
	}()
	receiveRetryResult(t, entered)
	_, err := r.route(context.Background(), virtualKey{"worker", "two"}, s.Ref(), "payload")
	require.ErrorIs(t, err, errVirtualRouterBusy)
	require.NoError(t, s.Stop(time.Second))
	require.ErrorIs(t, receiveRetryResult(t, result), context.Canceled)
	require.Empty(t, r.slots)
	_, err = r.route(context.Background(), virtualKey{"worker", "one"}, s.Ref(), "payload")
	require.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
	require.Eventually(t, func() bool {
		_, size := routerCache(r, virtualKey{"worker", "one"})
		return size == 0
	}, time.Second, time.Millisecond)
}

func TestVirtualRouter_CanceledRefreshDoesNotSendAgain(t *testing.T) {
	s, r, b, _ := newRouterTest(t, 2, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var sends int
	b.deliverFn = func(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error) {
		sends++
		cancel()
		return virtual.DeliveryStale, nil
	}
	_, err := r.route(ctx, virtualKey{"worker", "one"}, s.Ref(), "payload")
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, sends)
}

func TestVirtualRouter_TCPMigrationUnknownAndNewCall(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	n.leader(t, 1, 2, 3)
	seen := make(chan routedObservation, 32)
	a, _ := newRoutedOwner(t, n, 4, seen)
	b, _ := newRoutedOwner(t, n, 5, seen)
	var loseReply atomic.Bool
	caller := newRemoteControlSystemAt(t, n, 6, func(p probeRequest) bool {
		_, receipt := p.message.(*virtual.DeliveryResult)
		return receipt && loseReply.Load()
	})
	client := remoteOwnershipClient(t, caller, n)
	r, err := newVirtualRouter(client, 4, 4)
	require.NoError(t, err)
	var reads, sends atomic.Int32
	r.backend = &routerTestBackend{
		lookupFn: func(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, bool, error) {
			reads.Add(1)
			return client.lookup(ctx, key)
		},
		deliverFn: func(ctx context.Context, value clusterstore.VirtualOwnership, sender vivid.ActorRef, message vivid.Message) (virtual.DeliveryStatus, error) {
			sends.Add(1)
			return client.deliver(ctx, value, sender, message)
		},
	}
	replies := make(chan string, 32)
	sender, err := caller.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if payload, ok := ctx.Message().(*Ref); ok {
			replies <- payload.String()
		}
	}))
	require.NoError(t, err)
	key := virtualKey{"worker", "one"}
	require.NoError(t, a.Ask(a.VirtualRef(key.kind, key.name), 1).Wait())
	checkBusiness := func() {
		t.Helper()
		require.Equal(t, routedObservation{sender.String(), caller.Ref().String()}, receiveRetryResult(t, seen))
		require.Equal(t, caller.Ref().String(), receiveRetryResult(t, replies))
	}
	for range 2 {
		status, err := r.route(context.Background(), key, sender, caller.Ref())
		require.NoError(t, err)
		require.Equal(t, virtual.DeliveryAccepted, status)
		checkBusiness()
	}
	require.EqualValues(t, 1, reads.Load(), "稳定缓存命中不重新查询发送端归属")
	// 另一个调用方缓存同一旧位置，稍后验证不可达时仅失效而不重发。
	unreachable, err := newVirtualRouter(client, 4, 4)
	require.NoError(t, err)
	_, err = unreachable.resolve(context.Background(), key)
	require.NoError(t, err)
	old := waitVirtualMailbox(t, a, "one")
	a.Kill(old.ctx.ref, false, "router migration")
	require.Eventually(t, func() bool {
		v := a.virtualCoordinator
		v.mu.Lock()
		defer v.mu.Unlock()
		return v.entries[key] == nil
	}, 2*time.Second, time.Millisecond)
	require.NoError(t, b.Ask(b.VirtualRef(key.kind, key.name), 1).Wait())
	before := sends.Load()
	status, err := r.route(context.Background(), key, sender, caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	require.Equal(t, before+2, sends.Load(), "旧入口明确拒绝后，新代次仅再发送一次")
	require.EqualValues(t, 2, reads.Load())
	checkBusiness()
	n.partition(4)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	_, err = unreachable.route(ctx, key, sender, caller.Ref())
	cancel()
	require.ErrorIs(t, err, errVirtualDeliveryUnknown, "旧地址不可达不能证明旧消息未接纳")
	select {
	case <-seen:
		t.Fatal("未知投递不能自动重发到新所有者")
	default:
	}
	status, err = unreachable.route(context.Background(), key, sender, caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status, "后续显式新调用重新解析新位置")
	checkBusiness()
	n.partition()
	// 丢失实际 Accepted 回执同样返回未知，不因存在可查询的新记录就重试。
	loseReply.Store(true)
	before = sends.Load()
	ctx, cancel = context.WithTimeout(context.Background(), 150*time.Millisecond)
	_, err = r.route(ctx, key, sender, caller.Ref())
	cancel()
	require.ErrorIs(t, err, errVirtualDeliveryUnknown)
	require.Equal(t, before+1, sends.Load())
	checkBusiness()
	loseReply.Store(false)
	status, err = r.route(context.Background(), key, sender, caller.Ref())
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status)
	require.EqualValues(t, 3, reads.Load())
	checkBusiness()
	select {
	case <-seen:
		t.Fatal("存在重复的业务投递")
	default:
	}
}

func TestVirtualRouter_InvalidInputDoesNotLookupOrDeliver(t *testing.T) {
	s, r, b, value := newRouterTest(t, 2, 2)
	b.lookupFn = func(context.Context, virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		t.Fatal("非法输入不能查询归属")
		return value, true, nil
	}
	key := virtualKey{"worker", "one"}
	for _, payload := range []any{nil, (*Ref)(nil), new(virtual.RoutedDelivery)} {
		_, err := r.route(context.Background(), key, s.Ref(), payload)
		require.ErrorIs(t, err, virtual.ErrInvalidRoutedDelivery)
	}
	_, err := r.route(context.Background(), key, nil, "payload")
	require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
	_, err = r.route(context.Background(), key, (*Ref)(nil), "payload")
	require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
	_, err = r.route(context.Background(), key, s.VirtualRef("worker", "sender"), "payload")
	require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
	_, err = r.route(context.Background(), virtualKey{}, s.Ref(), "payload")
	require.ErrorIs(t, err, clusterstore.ErrInvalidVirtualOwnership)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = r.route(ctx, key, s.Ref(), "payload")
	require.ErrorIs(t, err, context.Canceled)
	// 即使控制协议返回结构完整的记录，也必须匹配本次逻辑身份。
	b.lookupFn = func(context.Context, virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		value.Name = "different"
		return value, true, nil
	}
	b.deliverFn = func(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error) {
		t.Fatal("错误身份的记录不能投递")
		return virtual.DeliveryAccepted, nil
	}
	_, err = r.route(context.Background(), key, s.Ref(), "payload")
	require.ErrorIs(t, err, clusterstore.ErrInvalidVirtualOwnership)
	_, size := routerCache(r, key)
	require.Zero(t, size)
}

func TestVirtualRouter_PreservesCallerDeadlineAndReleasesCanceledSlot(t *testing.T) {
	s, r, b, value := newRouterTest(t, 2, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	want, _ := ctx.Deadline()
	entered := make(chan struct{}, 1)
	var once atomic.Bool
	b.lookupFn = func(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		if once.CompareAndSwap(false, true) {
			deadline, _ := ctx.Deadline()
			if deadline != want {
				return value, false, errors.New("caller deadline was reset")
			}
			entered <- struct{}{}
			<-ctx.Done()
			return value, false, ctx.Err()
		}
		return value, true, nil
	}
	result := make(chan error, 1)
	go func() {
		_, err := r.route(ctx, virtualKey{"worker", "one"}, s.Ref(), "payload")
		result <- err
	}()
	receiveRetryResult(t, entered)
	cancel()
	require.ErrorIs(t, receiveRetryResult(t, result), context.Canceled)
	status, err := r.route(context.Background(), virtualKey{"worker", "one"}, s.Ref(), "payload")
	require.NoError(t, err)
	require.Equal(t, virtual.DeliveryAccepted, status, "已取消调用及时归还并发名额")
}

func TestVirtualRouter_CanceledLookupCannotPublishLateSuccess(t *testing.T) {
	s, r, b, value := newRouterTest(t, 2, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.lookupFn = func(context.Context, virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		cancel()
		return value, true, nil
	}
	b.deliverFn = func(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error) {
		t.Fatal("取消后返回的成功查询不能继续投递")
		return virtual.DeliveryAccepted, nil
	}
	key := virtualKey{"worker", "one"}
	_, err := r.route(ctx, key, s.Ref(), "payload")
	require.ErrorIs(t, err, context.Canceled)
	_, size := routerCache(r, key)
	require.Zero(t, size)
	require.Empty(t, r.slots)
}

func TestVirtualRouter_ConcurrentEvictionKeepsCacheBounded(t *testing.T) {
	s, r, b, value := newRouterTest(t, 8, 16)
	b.lookupFn = func(_ context.Context, key virtualKey) (clusterstore.VirtualOwnership, bool, error) {
		v := value
		v.Name = key.name
		return v, true, nil
	}
	var workers sync.WaitGroup
	failures := make(chan error, 8)
	for worker := range 8 {
		workers.Go(func() {
			for index := range 64 {
				key := virtualKey{"worker", strconv.Itoa((worker*17 + index) % 32)}
				status, err := r.route(context.Background(), key, s.Ref(), "payload")
				if err != nil || status != virtual.DeliveryAccepted {
					failures <- errors.New("concurrent route failed")
					return
				}
			}
		})
	}
	workers.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	r.mu.Lock()
	count, recency := len(r.cache), r.recent.Len()
	r.mu.Unlock()
	require.LessOrEqual(t, count, 8)
	require.Equal(t, count, recency, "并发替换和淘汰须保持两个索引一致")
	require.Empty(t, r.slots)
}
