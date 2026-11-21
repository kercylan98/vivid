package actor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

type ownershipTestBackend struct {
	virtualOwnershipBackend
	claimFn    func(context.Context, virtualKey) (clusterstore.VirtualOwnership, error)
	validateFn func(context.Context, clusterstore.VirtualOwnership) error
	releaseFn  func(context.Context, clusterstore.VirtualOwnership) error
}

func (b *ownershipTestBackend) claim(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, error) {
	if b.claimFn != nil {
		return b.claimFn(ctx, key)
	}
	return b.virtualOwnershipBackend.claim(ctx, key)
}
func (b *ownershipTestBackend) validate(ctx context.Context, ownership clusterstore.VirtualOwnership) error {
	if b.validateFn != nil {
		return b.validateFn(ctx, ownership)
	}
	return b.virtualOwnershipBackend.validate(ctx, ownership)
}
func (b *ownershipTestBackend) release(ctx context.Context, ownership clusterstore.VirtualOwnership) error {
	if b.releaseFn != nil {
		return b.releaseFn(ctx, ownership)
	}
	return b.virtualOwnershipBackend.release(ctx, ownership)
}

func newOwnershipClient(t *testing.T, n *raftRuntimeNetwork, leader uint64) *virtualOwnershipClient {
	t.Helper()
	owner, err := clusterstore.NewVirtualOwner("owner.example:9000")
	require.NoError(t, err)
	client, err := newVirtualOwnershipClient(n.systems[leader], n.refs[leader], owner)
	require.NoError(t, err)
	return client
}

func newOwnedVirtualSystem(t *testing.T, backend virtualOwnershipBackend, timeout time.Duration, provider vivid.VirtualActorProvider, options vivid.VirtualActorOptions) *System {
	t.Helper()
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		vivid.WithActorSystemVirtualActorProvider("worker", provider), vivid.WithActorSystemVirtualActorOptions(options))
	require.NoError(t, system.virtualCoordinator.configureOwnership(backend, timeout))
	require.NoError(t, system.Start())
	t.Cleanup(func() {
		if !system.requestsStopped() {
			require.NoError(t, system.Stop(3*time.Second))
		}
	})
	return system
}

func TestOwnedVirtual_CompetitionPassivationAndReactivation(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	var aCalls, bCalls atomic.Int32
	a := newOwnedVirtualSystem(t, newOwnershipClient(t, n, leader), time.Second, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		aCalls.Add(1)
		return vivid.ActorFN(virtualReply)
	}), vivid.VirtualActorOptions{IdleTimeout: time.Hour})
	b := newOwnedVirtualSystem(t, newOwnershipClient(t, n, leader), time.Second, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		bCalls.Add(1)
		return vivid.ActorFN(virtualReply)
	}), vivid.VirtualActorOptions{})
	ref := a.VirtualRef("worker", "one")
	var requests []vivid.Future[vivid.Message]
	for range 32 {
		requests = append(requests, a.Ask(ref, 1))
	}
	var physical vivid.ActorRef
	for _, request := range requests {
		result, err := request.Result()
		require.NoError(t, err)
		if physical == nil {
			physical = result.(vivid.ActorRef)
		}
		require.Same(t, physical, result)
	}
	require.EqualValues(t, 1, aCalls.Load())
	require.ErrorIs(t, b.Ask(b.VirtualRef("worker", "one"), 1).Wait(), clusterstore.ErrVirtualOwnershipConflict)
	require.Zero(t, bCalls.Load(), "没有归属时不得调用 Provider")
	first, exists := n.ownership(t, leader, "worker", "one")
	require.True(t, exists)
	old := waitVirtualMailbox(t, a, "one")
	expireVirtualMailbox(old)
	require.Eventually(t, func() bool {
		v := a.virtualCoordinator
		v.mu.Lock()
		defer v.mu.Unlock()
		return len(v.entries) == 0
	}, time.Second, time.Millisecond)
	require.NoError(t, b.Ask(b.VirtualRef("worker", "one"), 1).Wait())
	next, exists := n.ownership(t, leader, "worker", "one")
	require.True(t, exists)
	require.Greater(t, next.Generation, first.Generation)
	require.NotEqual(t, next.Owner.InstanceID, first.Owner.InstanceID)
	old.release()
	current, exists := n.ownership(t, leader, "worker", "one")
	require.True(t, exists)
	require.Equal(t, next, current, "旧本地清理不可释放其他系统的新归属")
	require.NoError(t, b.Stop(time.Second))
	_, exists = n.ownership(t, leader, "worker", "one")
	require.False(t, exists)
}

func TestOwnedVirtual_FailedInitializationReleasesOwnership(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	for _, failure := range []string{"nil", "panic", "prelaunch"} {
		t.Run(failure, func(t *testing.T) {
			var calls atomic.Int32
			system := newOwnedVirtualSystem(t, newOwnershipClient(t, n, leader), time.Second, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
				if calls.Add(1) == 1 {
					switch failure {
					case "nil":
						return nil
					case "panic":
						panic("provider")
					default:
						return vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error { return context.Canceled }, vivid.ActorFN(virtualReply))
					}
				}
				return vivid.ActorFN(virtualReply)
			}), vivid.VirtualActorOptions{})
			ref := system.VirtualRef("worker", failure)
			require.ErrorIs(t, system.Ask(ref, 1).Wait(), vivid.ErrorActorSpawnFailed)
			require.NoError(t, system.Ask(ref, 1).Wait(), "新投递等释放结束后重新取得归属")
			require.EqualValues(t, 2, calls.Load())
		})
	}
}

func TestOwnedVirtual_RechecksBeforeProviderAndAfterPrelaunch(t *testing.T) {
	for _, stage := range []string{"claim", "prelaunch"} {
		t.Run(stage, func(t *testing.T) {
			n := newRaftRuntimeNetwork(t, false, nil)
			leader := n.leader(t, 1, 2, 3)
			client := newOwnershipClient(t, n, leader)
			entered, release := make(chan clusterstore.VirtualOwnership, 1), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			backend := &ownershipTestBackend{virtualOwnershipBackend: client}
			backend.claimFn = func(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, error) {
				ownership, err := client.claim(ctx, key)
				if err == nil {
					entered <- ownership
					if stage == "claim" {
						<-release
					}
				}
				return ownership, err
			}
			var providers, launches atomic.Int32
			system := newOwnedVirtualSystem(t, backend, time.Second, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
				providers.Add(1)
				return vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error {
					if stage == "prelaunch" {
						<-release
					}
					return nil
				}, vivid.ActorFN(func(ctx vivid.ActorContext) {
					if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
						launches.Add(1)
					}
					virtualReply(ctx)
				}))
			}), vivid.VirtualActorOptions{})
			pending := system.Ask(system.VirtualRef("worker", "one"), 1)
			claimed := receiveRetryResult(t, entered)
			nextOwner, err := clusterstore.NewVirtualOwner("next.example:9000")
			require.NoError(t, err)
			change, err := clusterstore.ClaimVirtualOwnership("worker", "one", nextOwner, claimed.Generation)
			require.NoError(t, err)
			next, _, err := n.changeOwnership(t, leader, change)
			require.NoError(t, err)
			unblock()
			require.ErrorIs(t, pending.Wait(), clusterstore.ErrVirtualOwnershipConflict)
			if stage == "claim" {
				require.Zero(t, providers.Load())
			}
			require.Zero(t, launches.Load(), "失去归属后不能开启业务邮箱")
			require.NoError(t, system.Stop(time.Second))
			current, exists := n.ownership(t, leader, "worker", "one")
			require.True(t, exists)
			require.Equal(t, next, current)
		})
	}
}

func TestOwnedVirtual_UnknownClaimNeverCreatesActor(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	client := newOwnershipClient(t, n, leader)
	n.partition(leader)
	var calls atomic.Int32
	system := newOwnedVirtualSystem(t, client, 80*time.Millisecond, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		calls.Add(1)
		return vivid.ActorFN(virtualReply)
	}), vivid.VirtualActorOptions{})
	require.ErrorIs(t, system.Ask(system.VirtualRef("worker", "one"), 1).Wait(), clusterstore.ErrRaftOutcomeUnknown)
	require.Zero(t, calls.Load())
	require.NoError(t, system.Stop(time.Second))
	n.partition()
	require.Zero(t, calls.Load(), "恢复网络不会自动重试未知写入或转为本地激活")
}

func TestOwnedVirtual_StopCancelsClaimAndReleasesLateSuccess(t *testing.T) {
	for _, late := range []bool{false, true} {
		t.Run(fmt.Sprint(late), func(t *testing.T) {
			n := newRaftRuntimeNetwork(t, false, nil)
			leader := n.leader(t, 1, 2, 3)
			client := newOwnershipClient(t, n, leader)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			backend := &ownershipTestBackend{virtualOwnershipBackend: client}
			backend.claimFn = func(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, error) {
				if !late {
					close(entered)
					<-ctx.Done()
					return clusterstore.VirtualOwnership{}, ctx.Err()
				}
				ownership, err := client.claim(ctx, key)
				close(entered)
				<-release
				return ownership, err
			}
			var calls atomic.Int32
			system := newOwnedVirtualSystem(t, backend, time.Second, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
				calls.Add(1)
				return vivid.ActorFN(virtualReply)
			}), vivid.VirtualActorOptions{})
			pending := system.Ask(system.VirtualRef("worker", "one"), 1, 0)
			receiveRetryResult(t, entered)
			if late {
				require.ErrorIs(t, system.Stop(time.Millisecond), vivid.ErrorActorSystemStopFailed)
				unblock()
			}
			require.NoError(t, system.Stop(2*time.Second))
			require.ErrorIs(t, pending.Wait(), vivid.ErrorActorSystemStopped)
			require.Zero(t, calls.Load())
			_, exists := n.ownership(t, leader, "worker", "one")
			require.False(t, exists)
		})
	}
}

func TestOwnedVirtual_StopAfterCreationWaitsForBusinessCleanup(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	client := newOwnershipClient(t, n, leader)
	spawned, cleanup := make(chan struct{}), make(chan struct{})
	spawnRelease, cleanupRelease := make(chan struct{}), make(chan struct{})
	var spawnOnce, cleanupOnce sync.Once
	unblockSpawn := func() { spawnOnce.Do(func() { close(spawnRelease) }) }
	unblockCleanup := func() { cleanupOnce.Do(func() { close(cleanupRelease) }) }
	defer unblockSpawn()
	defer unblockCleanup()
	var releases atomic.Int32
	backend := &ownershipTestBackend{virtualOwnershipBackend: client, releaseFn: func(ctx context.Context, o clusterstore.VirtualOwnership) error {
		releases.Add(1)
		return client.release(ctx, o)
	}}
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				if message, ok := ctx.Message().(*vivid.OnKilled); ok && message.Ref == ctx.Ref() {
					close(cleanup)
					<-cleanupRelease
				}
				virtualReply(ctx)
			})
		})))
	require.NoError(t, system.virtualCoordinator.configureOwnership(backend, time.Second))
	system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
		if event, ok := event.(ves.ActorSpawnedEvent); ok && event.ActorRef.GetPath() != "/" {
			close(spawned)
			<-spawnRelease
		}
	}}
	require.NoError(t, system.Start())
	t.Cleanup(func() {
		if !system.requestsStopped() {
			require.NoError(t, system.Stop(2*time.Second))
		}
	})
	pending := system.Ask(system.VirtualRef("worker", "one"), 1, 0)
	receiveRetryResult(t, spawned)
	require.ErrorIs(t, system.Stop(time.Millisecond), vivid.ErrorActorSystemStopFailed)
	unblockSpawn()
	receiveRetryResult(t, cleanup)
	require.Zero(t, releases.Load(), "实例尚未清理，不得释放已经取得的归属")
	_, exists := n.ownership(t, leader, "worker", "one")
	require.True(t, exists)
	unblockCleanup()
	require.NoError(t, system.Stop(2*time.Second))
	require.ErrorIs(t, pending.Wait(), vivid.ErrorActorSystemStopped)
	require.EqualValues(t, 1, releases.Load())
	_, exists = n.ownership(t, leader, "worker", "one")
	require.False(t, exists)
}

func TestOwnedVirtual_DeliveryWaitsForReleaseResult(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			n := newRaftRuntimeNetwork(t, false, nil)
			leader := n.leader(t, 1, 2, 3)
			client := newOwnershipClient(t, n, leader)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			var releases, providers atomic.Int32
			backend := &ownershipTestBackend{virtualOwnershipBackend: client, releaseFn: func(ctx context.Context, o clusterstore.VirtualOwnership) error {
				if releases.Add(1) == 1 {
					close(entered)
					<-release
					if failed {
						return clusterstore.ErrRaftOutcomeUnknown
					}
				}
				return client.release(ctx, o)
			}}
			system := newOwnedVirtualSystem(t, backend, time.Second, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
				providers.Add(1)
				return vivid.ActorFN(virtualReply)
			}), vivid.VirtualActorOptions{IdleTimeout: time.Hour})
			ref := system.VirtualRef("worker", "one")
			require.NoError(t, system.Ask(ref, 1).Wait())
			expireVirtualMailbox(waitVirtualMailbox(t, system, "one"))
			receiveRetryResult(t, entered)
			pending := system.Ask(ref, 2)
			system.virtualCoordinator.mu.Lock()
			queued := system.virtualCoordinator.pending
			system.virtualCoordinator.mu.Unlock()
			require.Equal(t, 1, queued)
			require.EqualValues(t, 1, providers.Load(), "释放完成前不能创建新实例")
			unblock()
			if failed {
				require.ErrorIs(t, pending.Wait(), clusterstore.ErrRaftOutcomeUnknown)
				require.EqualValues(t, 1, providers.Load(), "释放结果未知时不能恢复本地创建")
			} else {
				require.NoError(t, pending.Wait())
				require.EqualValues(t, 2, providers.Load())
			}
		})
	}
}

func TestOwnedVirtual_StopSharesReleaseBudgetAndConcurrency(t *testing.T) {
	const count, concurrency = 12, 2
	deadlines := make(chan time.Time, count)
	var active, maximum atomic.Int32
	backend := &ownershipTestBackend{
		claimFn: func(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, error) {
			return clusterstore.VirtualOwnership{Kind: key.kind, Name: key.name, Generation: 1}, nil
		},
		validateFn: func(context.Context, clusterstore.VirtualOwnership) error { return nil },
		releaseFn: func(ctx context.Context, _ clusterstore.VirtualOwnership) error {
			current := active.Add(1)
			defer active.Add(-1)
			for previous := maximum.Load(); current > previous; previous = maximum.Load() {
				if maximum.CompareAndSwap(previous, current) {
					break
				}
			}
			deadline, _ := ctx.Deadline()
			deadlines <- deadline
			<-ctx.Done()
			return ctx.Err()
		},
	}
	// 用可控后端验证队列调度，真实 CAS 与网络错误由其他用例覆盖。
	system := newOwnedVirtualSystem(t, backend, 100*time.Millisecond, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		return vivid.ActorFN(virtualReply)
	}), vivid.VirtualActorOptions{ActivationConcurrency: concurrency, IdleTimeout: time.Hour})
	for i := range count {
		require.NoError(t, system.Ask(system.VirtualRef("worker", fmt.Sprint(i)), 1).Wait())
	}
	require.NoError(t, system.Stop(3*time.Second))
	require.EqualValues(t, concurrency, maximum.Load())
	require.Zero(t, active.Load())
	for range count {
		require.Equal(t, system.virtualCoordinator.gate.deadline, receiveRetryResult(t, deadlines), "队列后部任务不得重新取得完整超时")
	}
}

func TestOwnedVirtual_RestartValidatesOwnership(t *testing.T) {
	for _, stage := range []string{"success", "provider", "prelaunch"} {
		t.Run(stage, func(t *testing.T) {
			n := newRaftRuntimeNetwork(t, false, nil)
			leader := n.leader(t, 1, 2, 3)
			client := newOwnershipClient(t, n, leader)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			var providers atomic.Int32
			launched := make(chan struct{}, 2)
			system := newOwnedVirtualSystem(t, client, time.Second, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
				generation := providers.Add(1)
				return vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error {
					if stage == "prelaunch" && generation == 2 {
						close(entered)
						<-release
					}
					return nil
				}, vivid.ActorFN(func(ctx vivid.ActorContext) {
					if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
						launched <- struct{}{}
					}
					virtualReply(ctx)
				}))
			}), vivid.VirtualActorOptions{IdleTimeout: time.Hour})
			ref := system.VirtualRef("worker", "one")
			physical, err := system.Ask(ref, 1).Result()
			require.NoError(t, err)
			receiveRetryResult(t, launched)
			first, exists := n.ownership(t, leader, "worker", "one")
			require.True(t, exists)
			transfer := func() clusterstore.VirtualOwnership {
				owner, err := clusterstore.NewVirtualOwner("next.example:9000")
				require.NoError(t, err)
				change, err := clusterstore.ClaimVirtualOwnership("worker", "one", owner, first.Generation)
				require.NoError(t, err)
				next, _, err := n.changeOwnership(t, leader, change)
				require.NoError(t, err)
				return next
			}
			var next clusterstore.VirtualOwnership
			if stage == "provider" {
				next = transfer()
			}
			system.AdvancedTell(true, ref, &RestartMessage{Reason: "ownership validation"})
			if stage == "prelaunch" {
				receiveRetryResult(t, entered)
				next = transfer()
				unblock()
			}
			if stage == "success" {
				receiveRetryResult(t, launched)
				require.NoError(t, system.Ask(ref, 2).Wait())
				current, exists := n.ownership(t, leader, "worker", "one")
				require.True(t, exists)
				require.Equal(t, first, current, "原地重启保留同一归属代次")
			} else {
				// 框架 Ping 在同一邮箱中越过重启收尾，业务不得恢复。
				require.NoError(t, system.Ask(physical.(vivid.ActorRef), new(messages.PingMessage), time.Second).Wait())
				require.Empty(t, launched)
				require.False(t, system.HeartbeatProbe(physical.(vivid.ActorRef)).Available)
			}
			if stage == "provider" {
				require.EqualValues(t, 1, providers.Load())
			} else {
				require.EqualValues(t, 2, providers.Load())
			}
			require.NoError(t, system.Stop(2*time.Second))
			current, exists := n.ownership(t, leader, "worker", "one")
			require.Equal(t, stage != "success", exists)
			if exists {
				require.Equal(t, next, current, "旧实例停止不能删除新所有者")
			}
		})
	}
}

func TestOwnedVirtual_ConfigurationIsFixedBeforeStart(t *testing.T) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	ref, err := NewRef(LocalAddress, "/raft")
	require.NoError(t, err)
	owner, err := clusterstore.NewVirtualOwner("owner.example:9000")
	require.NoError(t, err)
	_, err = newVirtualOwnershipClient(system, ref, owner)
	require.ErrorIs(t, err, vivid.ErrorIllegalArgument, "未启动控制系统不能作为入口")
	backend := &ownershipTestBackend{}
	v := system.virtualCoordinator
	require.ErrorIs(t, v.configureOwnership(nil, 0), vivid.ErrorIllegalArgument)
	require.ErrorIs(t, v.configureOwnership(backend, -1), vivid.ErrorIllegalArgument)
	require.NoError(t, v.configureOwnership(backend, 0))
	require.Equal(t, 5*time.Second, v.gate.timeout)
	require.ErrorIs(t, v.configureOwnership(backend, 0), vivid.ErrorIllegalArgument)
	require.NoError(t, system.Start())
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	require.ErrorIs(t, v.configureOwnership(backend, time.Second), vivid.ErrorIllegalArgument)
}
