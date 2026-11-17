package actor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/bridge"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func TestContext_ShutdownRequestsLifetime(t *testing.T) {
	for _, phase := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "phase"}[phase], func(t *testing.T) {
			system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			killed := make(chan vivid.ActorRef, 16)
			system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
				if event, ok := event.(ves.ActorKilledEvent); ok {
					killed <- event.ActorRef
				}
			}}
			require.NoError(t, system.Start())
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if message, ok := ctx.Message().(int); ok {
					ctx.Reply(message)
				}
			}))
			require.NoError(t, err)
			type requests struct {
				ask, task vivid.Future[vivid.Message]
			}
			pending := make(chan requests, 2)
			release, finished := make(chan struct{}), make(chan struct{}, 2)
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			completed := make(chan struct{})
			launched := make(chan *Context, 1)
			behavior := vivid.Behavior(func(ctx vivid.ActorContext) {
				switch message := ctx.Message().(type) {
				case *vivid.OnLaunch:
					launched <- ctx.(*Context)
					return
				case *vivid.OnKill:
					if phase {
						close(completed)
					}
				case *vivid.OnKilled:
					if !message.Ref.Equals(ctx.Ref()) {
						return
					}
				default:
					return
				}
				// 两个收尾钩子内均保留正常请求能力，未完成的请求随本次收尾结束。
				if err := ctx.Ask(target, 1, time.Second).Wait(); err != nil {
					panic(err)
				}
				if err := ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
					return "ready", nil
				})).Wait(); err != nil {
					panic(err)
				}
				pending <- requests{
					ask: ctx.Ask(target, "pending", 0),
					task: ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
						<-release
						defer func() { finished <- struct{}{} }()
						return "late result", nil
					})),
				}
			})
			owner, err := system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
				if phase {
					return ctx.WithPhaseKill(completed, time.Second, behavior)
				}
				return nil
			}, vivid.ActorFN(behavior)))
			require.NoError(t, err)
			ctx := receiveRetryResult(t, launched)
			system.Kill(owner, false)
			for !receiveRetryResult(t, killed).Equals(owner) {
			}
			for range 2 {
				requests := receiveRetryResult(t, pending)
				require.ErrorIs(t, requests.ask.Wait(), vivid.ErrorActorDeaded)
				require.ErrorIs(t, requests.task.Wait(), vivid.ErrorActorDeaded)
			}
			system.futureLock.RLock()
			require.Empty(t, system.futureAgents[owner.GetPath()])
			system.futureLock.RUnlock()
			retained := 0
			system.actorContexts.Range(func(_, value any) bool {
				if _, ok := value.(*future.Future[vivid.Message]); ok {
					retained++
				}
				return true
			})
			require.Zero(t, retained, "Actor 结束后两张索引均不能保留请求")
			coordinator := &stoppedRequestCoordinator{}
			system.virtualCoordinator = coordinator
			var tasks atomic.Int32
			require.ErrorIs(t, ctx.Ask(target, "late request", 0).Wait(), vivid.ErrorActorDeaded)
			require.ErrorIs(t, ctx.Ask(system.VirtualRef("worker", "late"), "late request", 0).Wait(), vivid.ErrorActorDeaded)
			require.ErrorIs(t, ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
				tasks.Add(1)
				return nil, nil
			})).Wait(), vivid.ErrorActorDeaded)
			require.Zero(t, tasks.Load())
			require.Zero(t, coordinator.calls.Load())
			unblock()
			for range 2 {
				receiveRetryResult(t, finished)
			}
			require.NoError(t, system.Ask(target, 1, time.Second).Wait(), "单个 Actor 终止不能关闭系统请求")
		})
	}
}

func TestContext_RequestRegistrationGeneration(t *testing.T) {
	system := &System{futureAgents: make(map[vivid.ActorPath]map[vivid.ActorPath]*AgentRef)}
	ctx := &Context{system: system, ref: &Ref{address: LocalAddress, path: "/owner"}}
	generation := ctx.requestEpoch.Load()
	system.closeContextRequests(ctx)
	system.reopenContextRequests(ctx)
	agent, request, cleaned := newRegistrationFuture(t, system, 0)
	err := system.appendFuture(agent, request, ctx, generation)
	require.ErrorIs(t, err, vivid.ErrorActorDeaded, "重启不能接纳旧生命周期中尚未登记的请求")
	request.Close(err)
	receiveRetryResult(t, cleaned)
	requireFutureUnregistered(t, system, agent)

	for range 200 {
		agent, request, cleaned := newRegistrationFuture(t, system, 0)
		generation := ctx.requestEpoch.Load()
		start := make(chan struct{})
		var workers sync.WaitGroup
		workers.Add(2)
		go func() {
			defer workers.Done()
			<-start
			if err := system.appendFuture(agent, request, ctx, generation); err != nil {
				request.Close(err)
			}
		}()
		go func() {
			defer workers.Done()
			<-start
			system.closeContextRequests(ctx)
		}()
		close(start)
		workers.Wait()
		receiveRetryResult(t, cleaned)
		require.ErrorIs(t, request.Wait(), vivid.ErrorActorDeaded)
		requireFutureUnregistered(t, system, agent)
		system.reopenContextRequests(ctx)
	}
}

func TestContext_ShutdownRequestsWaitForChildren(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
	require.NoError(t, err)
	launched, childStopped := make(chan vivid.ActorRef, 1), make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	pending := make(chan vivid.Future[vivid.Message], 3)
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			child, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(*vivid.OnKill); ok {
					close(childStopped)
					<-release
				}
			}))
			if err != nil {
				panic(err)
			}
			launched <- child
		case *vivid.OnKill, *vivid.OnKilled:
			pending <- ctx.Ask(target, "pending", 0)
		}
	}))
	require.NoError(t, err)
	receiveRetryResult(t, launched)
	system.Kill(owner, false)
	first := receiveRetryResult(t, pending)
	receiveRetryResult(t, childStopped)
	require.False(t, first.(*future.Future[vivid.Message]).IsCompleted(), "等待子 Actor 结束仍属于收尾过程")
	unblock()
	child, self := receiveRetryResult(t, pending), receiveRetryResult(t, pending)
	for _, request := range []vivid.Future[vivid.Message]{first, child, self} {
		result := make(chan error, 1)
		go func() { result <- request.Wait() }()
		require.ErrorIs(t, receiveRetryResult(t, result), vivid.ErrorActorDeaded)
	}
}

func TestContext_VirtualShutdownRequestsLifetime(t *testing.T) {
	system := NewTestSystem(t,
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.ActorProviderFN(func() vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {})
		})),
	)
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	pending := make(chan vivid.Future[vivid.Message], 1)
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnKill); ok {
			pending <- ctx.Ask(system.VirtualRef("worker", "one"), "pending", 0)
		}
	}))
	require.NoError(t, err)
	system.Kill(owner, false)
	request := receiveRetryResult(t, pending)
	result := make(chan error, 1)
	go func() { result <- request.Wait() }()
	require.ErrorIs(t, receiveRetryResult(t, result), vivid.ErrorActorDeaded, "虚拟转发不能将请求归入系统根 Actor")
	require.ErrorIs(t, system.AskWithSender(owner, system.Ref(), "late", 0).Wait(), vivid.ErrorActorDeaded,
		"转发不能在发送方退出后重新登记")
}

func TestContext_RestartRequestsLifetime(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(int); ok {
			ctx.Reply(message)
		}
	}))
	require.NoError(t, err)
	launched, pending, initialized := make(chan *Context, 2), make(chan vivid.Future[vivid.Message], 4), make(chan error, 2)
	var restarted bool
	owner, err := system.ActorOf(vivid.NewComplexCombinationActor(
		vivid.NewRestartedActor(func(ctx vivid.RestartContext) error {
			restarted = true
			initialized <- ctx.(*Context).Ask(target, 1, time.Second).Wait()
			return nil
		}),
		vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
			if restarted {
				initialized <- ctx.(*Context).Ask(target, 1, time.Second).Wait()
				pending <- ctx.(*Context).Ask(target, "new generation", 0)
			}
			return nil
		}),
		vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch message := ctx.Message().(type) {
			case *vivid.OnLaunch:
				launched <- ctx.(*Context)
			case *vivid.OnKill:
				pending <- ctx.Ask(target, "on kill", 0)
			case *vivid.OnKilled:
				if message.Ref.Equals(ctx.Ref()) {
					pending <- ctx.Ask(target, "on killed", 0)
				}
			}
		}),
	))
	require.NoError(t, err)
	ctx := receiveRetryResult(t, launched)
	ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, owner, owner, &RestartMessage{Reason: "request lifetime"}))
	oldKill, oldKilled := receiveRetryResult(t, pending), receiveRetryResult(t, pending)
	current := receiveRetryResult(t, pending)
	defer current.Close(context.Canceled)
	require.NoError(t, receiveRetryResult(t, initialized))
	require.NoError(t, receiveRetryResult(t, initialized))
	require.Same(t, ctx, receiveRetryResult(t, launched))
	require.ErrorIs(t, oldKill.Wait(), vivid.ErrorActorDeaded)
	require.ErrorIs(t, oldKilled.Wait(), vivid.ErrorActorDeaded)
	require.False(t, current.(*future.Future[vivid.Message]).IsCompleted(), "旧实例清理不能结束新实例的初始化请求")
}

type heldLifetimeCoordinator struct {
	bridge.VirtualCoordinator
	target  vivid.ActorRef
	entered chan struct{}
	release chan struct{}
}

func (c *heldLifetimeCoordinator) ResolveVirtual(recipient vivid.ActorRef) (vivid.ActorRef, error) {
	close(c.entered)
	<-c.release
	return c.target, nil
}

func TestContext_VirtualRequestAcrossRestart(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	var delivered atomic.Int32
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case string:
			delivered.Add(1)
			ctx.Reply("reply")
		case int:
			ctx.Reply("barrier")
		}
	}))
	require.NoError(t, err)
	launched := make(chan *Context, 2)
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
			launched <- ctx.(*Context)
		}
	}))
	require.NoError(t, err)
	ctx := receiveRetryResult(t, launched)
	coordinator := &heldLifetimeCoordinator{target: target, entered: make(chan struct{}), release: make(chan struct{})}
	system.virtualCoordinator = coordinator
	var once sync.Once
	resume := func() { once.Do(func() { close(coordinator.release) }) }
	defer resume()
	result := make(chan error, 1)
	go func() { result <- ctx.Ask(system.VirtualRef("worker", "one"), "pending", 0).Wait() }()
	receiveRetryResult(t, coordinator.entered)
	ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, owner, owner, &RestartMessage{Reason: "held virtual request"}))
	require.Same(t, ctx, receiveRetryResult(t, launched))
	resume()
	require.ErrorIs(t, receiveRetryResult(t, result), vivid.ErrorActorDeaded,
		"转发开始于旧生命周期，即使重启后才返回也必须结束等待")
	require.NoError(t, system.Ask(target, 1, time.Second).Wait())
	require.Zero(t, delivered.Load(), "旧请求应在投递前被拒绝，不能由新实例接纳")
}

func TestContext_VirtualRequestDefaultTimeout(t *testing.T) {
	system := NewTestSystem(t,
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemDefaultAskTimeout(0),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.ActorProviderFN(func() vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {})
		})),
	)
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	result := make(chan error, 1)
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
			result <- ctx.Ask(system.VirtualRef("worker", "one"), "pending").Wait()
		}
	}), vivid.WithActorDefaultAskTimeout(10*time.Millisecond))
	require.NoError(t, err)
	require.ErrorIs(t, receiveRetryResult(t, result), vivid.ErrorFutureTimeout,
		"虚拟请求沿用发起 Actor 的默认超时，不切换为根 Actor 的默认值")
}

func TestContext_RestartFailureClosesInitializationRequests(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
	require.NoError(t, err)
	pending := make(chan vivid.Future[vivid.Message], 1)
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}),
		vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor {
			return vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
				pending <- ctx.(*Context).Ask(target, "failed initialization", 0)
				return context.Canceled
			}, vivid.ActorFN(func(ctx vivid.ActorContext) {}))
		})),
	)
	require.NoError(t, err)
	system.AdvancedTell(true, owner, &RestartMessage{Reason: "failed initialization request"})
	request := receiveRetryResult(t, pending)
	result := make(chan error, 1)
	go func() { result <- request.Wait() }()
	require.ErrorIs(t, receiveRetryResult(t, result), vivid.ErrorActorDeaded)
}
