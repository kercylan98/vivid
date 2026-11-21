package actor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func newVirtualTestSystem(t *testing.T, provider vivid.VirtualActorProvider, options vivid.VirtualActorOptions) *TestSystem {
	t.Helper()
	system := NewTestSystem(t,
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("worker", provider),
		vivid.WithActorSystemVirtualActorOptions(options),
	)
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	return system
}

func virtualReply(ctx vivid.ActorContext) {
	if _, ok := ctx.Message().(int); ok {
		ctx.Reply(ctx.Ref())
	}
}

func waitVirtualMailbox(t *testing.T, system *System, name string) *virtualMailbox {
	t.Helper()
	var result *virtualMailbox
	require.Eventually(t, func() bool {
		v := system.virtualCoordinator
		v.mu.Lock()
		defer v.mu.Unlock()
		entry := v.entries[virtualKey{kind: "worker", name: name}]
		if entry != nil && !entry.creating && entry.mailbox != nil && entry.mailbox.pending == 0 {
			result = entry.mailbox
			return true
		}
		return false
	}, time.Second, time.Millisecond)
	return result
}

func expireVirtualMailbox(m *virtualMailbox) {
	v := m.coordinator
	v.mu.Lock()
	m.lastActivity = time.Now().Add(-2 * v.options.IdleTimeout)
	check := m.check
	v.mu.Unlock()
	m.Mailbox.Enqueue(mailbox.NewEnvelop(true, m.ctx.ref, m.ctx.ref, check))
}

func TestVirtual_ConcurrentActivation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls, received atomic.Int32
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	system := newVirtualTestSystem(t, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		calls.Add(1)
		close(entered)
		<-release
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			if _, ok := ctx.Message().(int); ok {
				received.Add(1)
				ctx.Reply(ctx.Ref())
			}
		})
	}), vivid.VirtualActorOptions{})
	defer unblock()
	ref := system.VirtualRef("worker", "one")
	first := system.Ask(ref, 0)
	receiveRetryResult(t, entered)
	requests := make(chan vivid.Future[vivid.Message], 64)
	var producers sync.WaitGroup
	for range 64 {
		producers.Go(func() { requests <- system.Ask(ref, 1) })
	}
	producers.Wait()
	require.EqualValues(t, 1, calls.Load())
	unblock()
	physical, err := first.Result()
	require.NoError(t, err)
	for range 64 {
		result, err := receiveRetryResult(t, requests).Result()
		require.NoError(t, err)
		require.Same(t, physical, result)
	}
	require.EqualValues(t, 65, received.Load())
}

func TestVirtual_IndependentAndReentrantInitialization(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var system *System
	system = NewSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("slow", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			close(entered)
			<-release
			return vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
				return system.Ask(system.VirtualRef("fast", "one"), 1).Wait()
			}, vivid.ActorFN(virtualReply))
		})),
		vivid.WithActorSystemVirtualActorProvider("fast", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			return vivid.ActorFN(virtualReply)
		})),
	)
	require.NoError(t, system.Start())
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	defer unblock()
	request := system.Ask(system.VirtualRef("slow", "one"), 1)
	receiveRetryResult(t, entered)
	require.NoError(t, system.Ask(system.VirtualRef("fast", "one"), 1).Wait(), "不同身份不等待慢 Provider")
	unblock()
	require.NoError(t, request.Wait(), "预启动重入其他身份不持有激活锁")
}

func TestVirtual_ActivationFailureRetry(t *testing.T) {
	for _, failure := range []string{"panic", "nil", "typed nil", "prelaunch"} {
		t.Run(failure, func(t *testing.T) {
			var calls atomic.Int32
			system := newVirtualTestSystem(t, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
				if calls.Add(1) == 1 {
					switch failure {
					case "panic":
						panic("provider")
					case "nil":
						return nil
					case "typed nil":
						return vivid.ActorFN(nil)
					case "prelaunch":
						return vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
							return context.Canceled
						}, vivid.ActorFN(virtualReply))
					}
				}
				return vivid.ActorFN(virtualReply)
			}), vivid.VirtualActorOptions{})
			ref := system.VirtualRef("worker", "one")
			require.ErrorIs(t, system.Ask(ref, 1).Wait(), vivid.ErrorActorSpawnFailed)
			require.NoError(t, system.Ask(ref, 1).Wait())
			require.EqualValues(t, 2, calls.Load())
		})
	}
}

func TestVirtual_ActivationBudgetAndStop(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	system := NewTestSystem(t,
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorOptions(vivid.VirtualActorOptions{ActivationConcurrency: 1, PendingMessageLimit: 2}),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			calls.Add(1)
			close(entered)
			<-release
			return vivid.ActorFN(virtualReply)
		})),
	)
	defer unblock()
	first := system.Ask(system.VirtualRef("worker", "one"), 1, 0)
	receiveRetryResult(t, entered)
	second := system.Ask(system.VirtualRef("worker", "two"), 1, 0)
	require.ErrorIs(t, system.Ask(system.VirtualRef("worker", "three"), 1).Wait(), vivid.ErrorVirtualPendingMessagesFull)
	require.ErrorIs(t, system.Stop(time.Millisecond), vivid.ErrorActorSystemStopFailed)
	require.ErrorIs(t, first.Wait(), vivid.ErrorActorSystemStopped)
	require.ErrorIs(t, second.Wait(), vivid.ErrorActorSystemStopped)
	require.EqualValues(t, 1, calls.Load())
	unblock()
	require.NoError(t, system.Stop(time.Second))
	v := system.virtualCoordinator
	v.mu.Lock()
	defer v.mu.Unlock()
	require.Empty(t, v.entries)
	require.Empty(t, v.waiting)
	require.Zero(t, v.pending)
	require.Zero(t, v.running)
}

func TestVirtual_AskTimeoutIncludesActivation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var received atomic.Int32
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	system := newVirtualTestSystem(t, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		close(entered)
		<-release
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			if ctx.Message() == "expired" {
				received.Add(1)
			}
			virtualReply(ctx)
		})
	}), vivid.VirtualActorOptions{})
	defer unblock()
	request := system.Ask(system.VirtualRef("worker", "one"), "expired", 5*time.Millisecond)
	receiveRetryResult(t, entered)
	require.ErrorIs(t, request.Wait(), vivid.ErrorFutureTimeout)
	unblock()
	require.NoError(t, system.Ask(system.VirtualRef("worker", "one"), 1).Wait())
	require.Zero(t, received.Load())
}

func TestVirtual_PassivationWaitsForCleanup(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle", true: "explicit"}[explicit], func(t *testing.T) {
			killed, release := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			system := newVirtualTestSystem(t, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
				generation := calls.Add(1)
				return vivid.ActorFN(func(ctx vivid.ActorContext) {
					if message, ok := ctx.Message().(*vivid.OnKilled); ok && message.Ref == ctx.Ref() && generation == 1 {
						close(killed)
						<-release
					}
					virtualReply(ctx)
				})
			}), vivid.VirtualActorOptions{IdleTimeout: time.Hour})
			defer unblock()
			ref := system.VirtualRef("worker", "one")
			first, err := system.Ask(ref, 1).Result()
			require.NoError(t, err)
			old := waitVirtualMailbox(t, system.System, "one")
			oldCheck := old.check
			if explicit {
				system.Kill(first.(vivid.ActorRef), false)
			} else {
				expireVirtualMailbox(old)
			}
			receiveRetryResult(t, killed)
			pending := system.Ask(ref, 2)
			require.EqualValues(t, 1, calls.Load(), "旧 OnKilled 未结束时不能启动新实例")
			unblock()
			second, err := pending.Result()
			require.NoError(t, err)
			require.False(t, first.(vivid.ActorRef).Equals(second.(vivid.ActorRef)))
			current := waitVirtualMailbox(t, system.System, "one")
			current.Mailbox.Enqueue(mailbox.NewEnvelop(true, current.ctx.ref, current.ctx.ref, oldCheck))
			old.release()
			third, err := system.Ask(ref, 3).Result()
			require.NoError(t, err)
			require.Same(t, second, third, "旧钝化消息和重复清理不能影响新实例")
		})
	}
}

func TestVirtual_IdleTimerAndDisabledPassivation(t *testing.T) {
	killed := make(chan vivid.ActorRef, 2)
	system := newVirtualTestSystem(t, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			if message, ok := ctx.Message().(*vivid.OnKilled); ok && message.Ref == ctx.Ref() {
				killed <- ctx.Ref()
			}
			virtualReply(ctx)
		})
	}), vivid.VirtualActorOptions{IdleTimeout: 5 * time.Millisecond})
	ref := system.VirtualRef("worker", "one")
	first, err := system.Ask(ref, 1).Result()
	require.NoError(t, err)
	require.Same(t, first, receiveRetryResult(t, killed))
	second, err := system.Ask(ref, 1).Result()
	require.NoError(t, err)
	require.False(t, first.(vivid.ActorRef).Equals(second.(vivid.ActorRef)))

	disabled := newVirtualTestSystem(t, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		return vivid.ActorFN(virtualReply)
	}), vivid.VirtualActorOptions{IdleTimeout: -1})
	require.NoError(t, disabled.Ask(disabled.VirtualRef("worker", "one"), 1).Wait())
	m := waitVirtualMailbox(t, disabled.System, "one")
	require.Nil(t, m.timer)
}

func TestVirtual_PausedMailboxKeepsPendingMessages(t *testing.T) {
	system := newVirtualTestSystem(t, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		return vivid.ActorFN(virtualReply)
	}), vivid.VirtualActorOptions{IdleTimeout: time.Hour})
	ref := system.VirtualRef("worker", "one")
	require.NoError(t, system.Ask(ref, 1).Wait())
	m := waitVirtualMailbox(t, system.System, "one")
	m.Pause()
	request := system.Ask(ref, 2)
	expireVirtualMailbox(m)
	m.Resume()
	result, err := request.Result()
	require.NoError(t, err)
	require.Same(t, m.ctx.ref, result)
}

func TestVirtual_IdentityKeyAndInvalidRecipient(t *testing.T) {
	var calls atomic.Int32
	provider := vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		calls.Add(1)
		return vivid.ActorFN(virtualReply)
	})
	system := NewTestSystem(t,
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("a/b", provider),
		vivid.WithActorSystemVirtualActorProvider("a", provider),
	)
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	first, err := system.Ask(system.VirtualRef("a/b", "c"), 1).Result()
	require.NoError(t, err)
	second, err := system.Ask(system.VirtualRef("a", "b/c"), 1).Result()
	require.NoError(t, err)
	require.NotSame(t, first, second)
	require.EqualValues(t, 2, calls.Load())
	require.ErrorIs(t, system.Ask(system.VirtualRef("", "one"), 1).Wait(), vivid.ErrorIllegalArgument)
	require.ErrorIs(t, system.Ask(system.VirtualRef("a", ""), 1).Wait(), vivid.ErrorIllegalArgument)
	require.ErrorIs(t, system.Ask(system.VirtualRef("missing", "one"), 1).Wait(), vivid.ErrorVirtualActorProviderNotFound)
}

type virtualLifecycleActor struct {
	generation int32
	fixed      *atomic.Int32
	prelaunch  *atomic.Int32
	preRestart *atomic.Int32
	restarted  *atomic.Int32
	launched   chan *Context
}

func (a *virtualLifecycleActor) FixedOptions(ctx vivid.FixedOptionContext) []vivid.ActorOption {
	a.fixed.Add(1)
	return []vivid.ActorOption{
		vivid.WithActorName("business-name"),
		vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { panic("must use virtual provider") })),
		vivid.WithActorDefaultAskTimeout(13 * time.Millisecond),
	}
}

func (a *virtualLifecycleActor) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	a.prelaunch.Add(1)
	return nil
}

func (a *virtualLifecycleActor) OnPreRestart(ctx vivid.RestartContext) error {
	a.preRestart.Add(1)
	return nil
}

func (a *virtualLifecycleActor) OnRestarted(ctx vivid.RestartContext) error {
	a.restarted.Add(1)
	return nil
}

func (a *virtualLifecycleActor) OnReceive(ctx vivid.ActorContext) {
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.launched <- ctx.(*Context)
	case int:
		ctx.Reply(a.generation)
	}
}

func TestVirtual_NativeLifecycleAndRestart(t *testing.T) {
	var providers, fixed, prelaunch, preRestart, restarted atomic.Int32
	launched := make(chan *Context, 3)
	identities := make(chan virtualKey, 3)
	system := newVirtualTestSystem(t, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		identities <- virtualKey{kind: kind, name: name}
		return &virtualLifecycleActor{
			generation: providers.Add(1), fixed: &fixed,
			prelaunch: &prelaunch, preRestart: &preRestart, restarted: &restarted, launched: launched,
		}
	}), vivid.VirtualActorOptions{IdleTimeout: time.Hour})
	ref := system.VirtualRef("worker", "one")
	first, err := system.Ask(ref, 1).Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, first)
	require.Equal(t, virtualKey{kind: "worker", name: "one"}, receiveRetryResult(t, identities))
	ctx := receiveRetryResult(t, launched)
	require.Equal(t, 13*time.Millisecond, ctx.options.DefaultAskTimeout)
	require.NotEqual(t, "business-name", ctx.Name())
	m := waitVirtualMailbox(t, system.System, "one")
	oldCheck := m.check
	system.AdvancedTell(true, ref, &RestartMessage{Reason: "virtual restart"})
	require.Same(t, ctx, receiveRetryResult(t, launched))
	second, err := system.Ask(ref, 2).Result()
	require.NoError(t, err)
	require.EqualValues(t, 2, second)
	require.Equal(t, virtualKey{kind: "worker", name: "one"}, receiveRetryResult(t, identities))
	require.EqualValues(t, 1, fixed.Load())
	require.EqualValues(t, 2, prelaunch.Load())
	require.EqualValues(t, 1, preRestart.Load())
	require.EqualValues(t, 1, restarted.Load())
	current := waitVirtualMailbox(t, system.System, "one")
	require.Same(t, m, current)
	require.NotSame(t, oldCheck, current.check)
	current.Mailbox.Enqueue(mailbox.NewEnvelop(true, ctx.ref, ctx.ref, oldCheck))
	require.NoError(t, system.Ask(ref, 3).Wait())
}

func TestVirtual_InvalidConfiguration(t *testing.T) {
	for _, options := range []vivid.VirtualActorOptions{{ActivationConcurrency: -1}, {PendingMessageLimit: -1}} {
		system := NewSystem(
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
			vivid.WithActorSystemVirtualActorOptions(options),
		)
		require.ErrorIs(t, system.Start(), vivid.ErrorIllegalArgument)
		select {
		case <-system.stopCompleted:
		default:
			t.Fatal("配置失败须完成启动回滚")
		}
	}
}

func TestVirtual_StopWaitsForFinalRelease(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	system := NewSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			return vivid.ActorFN(virtualReply)
		})),
	)
	system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
		if event, ok := event.(ves.ActorKilledEvent); ok && event.ActorRef.GetPath() != "/" {
			close(entered)
			<-release
		}
	}}
	require.NoError(t, system.Start())
	defer unblock()
	require.NoError(t, system.Ask(system.VirtualRef("worker", "one"), 1).Wait())
	stop := make(chan error, 1)
	go func() { stop <- system.Stop(10 * time.Millisecond) }()
	receiveRetryResult(t, entered)
	require.ErrorIs(t, receiveRetryResult(t, stop), vivid.ErrorActorSystemStopFailed)
	select {
	case <-system.stopCompleted:
		t.Fatal("虚拟实例的事件发布和激活释放尚未结束，不能报告系统已关闭")
	default:
	}
	unblock()
	require.NoError(t, system.Stop(time.Second))
	system.virtualCoordinator.mu.Lock()
	defer system.virtualCoordinator.mu.Unlock()
	require.Empty(t, system.virtualCoordinator.entries)
}

func TestVirtual_BecomeStashAndSender(t *testing.T) {
	stashed, replayed := make(chan vivid.ActorRef, 1), make(chan vivid.ActorRef, 1)
	killed := make(chan struct{}, 1)
	unexpected := make(chan vivid.Message, 8)
	system := newVirtualTestSystem(t, vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message() {
			case "stash":
				ctx.Stash()
				stashed <- ctx.Sender()
			case "become":
				ctx.Become(func(ctx vivid.ActorContext) {
					switch ctx.Message() {
					case "restore":
						ctx.Unstash()
						ctx.Reply("restored")
					case "stash":
						replayed <- ctx.Sender()
					default:
						switch ctx.Message().(type) {
						case *vivid.OnKill:
							killed <- struct{}{}
						case *vivid.OnKilled:
						case int:
							ctx.Reply(ctx.Ref())
						default:
							unexpected <- ctx.Message()
						}
					}
				})
				ctx.Reply("changed")
			default:
				virtualReply(ctx)
			}
		})
	}), vivid.VirtualActorOptions{IdleTimeout: time.Hour})
	ref := system.VirtualRef("worker", "one")
	require.NoError(t, system.Ask(ref, 1).Wait())
	m := waitVirtualMailbox(t, system.System, "one")
	timer := m.timer
	system.TellWithSender(system.Ref(), ref, "stash")
	require.Same(t, system.Ref(), receiveRetryResult(t, stashed))
	require.NoError(t, system.Ask(ref, "become").Wait())
	waitVirtualMailbox(t, system.System, "one")
	require.Same(t, timer, m.timer, "普通消息只记录活跃时间，不重建空闲任务")
	expireVirtualMailbox(m)
	// 同一系统队列中的探测在空闲检查后执行，观察检查已完成且业务仍可用。
	require.NoError(t, system.Context.ask(true, ref, 1).Wait())
	require.NoError(t, system.Ask(ref, "restore").Wait())
	require.Same(t, system.Ref(), receiveRetryResult(t, replayed), "暂存保持原始消息与发送方")
	waitVirtualMailbox(t, system.System, "one")
	expireVirtualMailbox(m)
	receiveRetryResult(t, killed)
	require.Empty(t, unexpected, "内部空闲通知不能暴露给 Become 后的业务")
}
