package actor

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/scheduler"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

type initializationEvent struct{}

func TestContext_InitializationRollback(t *testing.T) {
	for scenario := range 4 {
		panicCallback, panicLogger := scenario&1 != 0, scenario&2 != 0
		t.Run(fmt.Sprintf("callback_panic=%t/logger_panic=%t", panicCallback, panicLogger), func(t *testing.T) {
			logger := log.NewTextLogger(log.WithLevel(log.LevelError))
			if panicLogger {
				logger = &eventStreamLogger{Logger: logger, observe: func(message string) {
					if message == "unsubscribed all events" {
						panic("cleanup logger panic")
					}
				}}
			}
			system := NewSystem(vivid.WithActorSystemLogger(logger))
			var lifecycle atomic.Int32
			system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
				var ref vivid.ActorRef
				switch event := event.(type) {
				case ves.ActorSpawnedEvent:
					ref = event.ActorRef
				case ves.ActorKilledEvent:
					ref = event.ActorRef
				}
				if ref != nil && ref.GetPath() == "/failed" {
					lifecycle.Add(1)
				}
			}}
			require.NoError(t, system.Start())
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
			require.NoError(t, err)
			// 其他 Actor 的订阅与调度任务不得被失败创建的回滚清理。
			system.EventStream().Subscribe(system, initializationEvent{})
			require.NoError(t, schedulerResultError(system.Scheduler().Once(target, time.Hour, "unrelated", vivid.WithSchedulerReference("unrelated"))))
			var failed *Context
			var pending *scheduler.Timer
			var ask, task vivid.Future[vivid.Message]
			release, finished := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			actor := vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
				failed = ctx.(*Context)
				ctx.EventStream().Subscribe(ctx, initializationEvent{})
				// 内部资源同样随初始化回滚；公开 PrelaunchContext 不提供调度及请求接口。
				if err := schedulerResultError(failed.scheduler.Load().Once(target, time.Hour, "pending", vivid.WithSchedulerReference("pending"))); err != nil {
					return err
				}
				pending = failed.scheduler.Load().tasks["pending"].timer
				ask = failed.Ask(target, "pending", 0)
				task = failed.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
					<-release
					defer close(finished)
					return "late result", nil
				}))
				if panicCallback {
					panic("prelaunch failed")
				}
				return context.Canceled
			}, vivid.ActorFN(func(ctx vivid.ActorContext) { panic("failed Actor must not launch") }))
			create := func() {
				ref, err := system.ActorOf(actor, vivid.WithActorName("failed"))
				require.Nil(t, ref)
				require.ErrorIs(t, err, vivid.ErrorActorSpawnFailed)
				require.ErrorIs(t, err, vivid.ErrorActorPrelaunchFailed)
				require.ErrorIs(t, err, context.Canceled)
			}
			if panicCallback {
				require.PanicsWithValue(t, "prelaunch failed", create, "回滚不改变用户 panic 的传播")
			} else {
				create()
			}
			require.NotNil(t, failed)
			es := system.eventStream.(*startupEventStream).EventStream.(*eventStream)
			es.mu.RLock()
			retained := len(es.subscriberTypes[failed.ref.GetPath()])
			_, subscribed := es.subscribers[reflect.TypeOf(initializationEvent{})][failed.ref.GetPath()]
			es.mu.RUnlock()
			require.Zero(t, retained)
			require.False(t, subscribed)
			es.mu.RLock()
			_, rootSubscribed := es.subscribers[reflect.TypeOf(initializationEvent{})][system.Ref().GetPath()]
			es.mu.RUnlock()
			require.True(t, rootSubscribed)
			require.NoError(t, system.Scheduler().Cancel("unrelated"))
			require.Empty(t, failed.scheduler.Load().tasks)
			require.False(t, pending.Stop(), "初始化回滚应已停止一次计时器")
			require.ErrorIs(t, ask.Wait(), vivid.ErrorActorDeaded)
			require.ErrorIs(t, task.Wait(), vivid.ErrorActorDeaded)
			require.ErrorIs(t, failed.Ask(target, "late", 0).Wait(), vivid.ErrorActorDeaded)
			_, exists := system.actorContexts.Load(failed.ref.GetPath())
			require.False(t, exists)
			system.actorInitLock.Lock()
			require.Empty(t, system.initializingActors)
			system.actorInitLock.Unlock()
			system.futureLock.RLock()
			require.Empty(t, system.futureAgents[failed.ref.GetPath()])
			system.futureLock.RUnlock()
			retained = 0
			system.actorContexts.Range(func(_, value any) bool {
				if _, ok := value.(*future.Future[vivid.Message]); ok {
					retained++
				}
				return true
			})
			require.Zero(t, retained)
			require.Zero(t, lifecycle.Load(), "创建失败不发布成功或死亡事件")
			unblock()
			receiveRetryResult(t, finished)
			require.ErrorIs(t, task.Wait(), vivid.ErrorActorDeaded)
			ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(string); ok {
					ctx.Reply("retry")
				}
			}), vivid.WithActorName("failed"))
			require.NoError(t, err, "失败后可重新使用同名")
			result, err := system.Ask(ref, "ready", time.Second).Result()
			require.NoError(t, err)
			require.Equal(t, "retry", result)
		})
	}
}

func TestContext_InitializationInvalidRefPreservesResources(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	system.EventStream().Subscribe(system, initializationEvent{})
	var called atomic.Int32
	_, err := NewContext(system.System, &Ref{address: "\x00", path: "/"}, vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
		called.Add(1)
		return nil
	}), vivid.WithActorName("invalid"))
	require.Error(t, err)
	require.Zero(t, called.Load())
	es := system.eventStream.(*eventStream)
	es.mu.RLock()
	_, exists := es.subscribers[reflect.TypeOf(initializationEvent{})][system.Ref().GetPath()]
	es.mu.RUnlock()
	require.True(t, exists, "引用尚未创建时不应按空路径回滚其他资源")
}

func TestContext_InitializationRootNameConflict(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	system.EventStream().Subscribe(system, initializationEvent{})
	var called atomic.Int32
	_, err := system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
		called.Add(1)
		return context.Canceled
	}), vivid.WithActorName(".."))
	require.ErrorIs(t, err, vivid.ErrorActorAlreadyExists, "根 Actor 不在普通路由索引中，仍须保留其名称")
	require.Zero(t, called.Load())
	es := system.eventStream.(*eventStream)
	es.mu.RLock()
	_, exists := es.subscribers[reflect.TypeOf(initializationEvent{})][system.Ref().GetPath()]
	es.mu.RUnlock()
	require.True(t, exists)
}

func TestContext_InitializationDuplicatePreservesResources(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	launched := make(chan *Context, 1)
	events := make(chan struct{}, 1)
	live, err := system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
		ctx.EventStream().Subscribe(ctx, initializationEvent{})
		return nil
	}, vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			if err := schedulerResultError(ctx.Scheduler().Once(ctx.Ref(), time.Hour, "pending", vivid.WithSchedulerReference("pending"))); err != nil {
				panic(err)
			}
			launched <- ctx.(*Context)
		case initializationEvent:
			events <- struct{}{}
		case string:
			ctx.Reply(ctx.Scheduler().Cancel("pending"))
		}
	})), vivid.WithActorName("live"))
	require.NoError(t, err)
	ctx := receiveRetryResult(t, launched)
	var called atomic.Int32
	ref, err := system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
		called.Add(1)
		ctx.EventStream().UnsubscribeAll(ctx)
		return context.Canceled
	}), vivid.WithActorName("live"))
	require.Nil(t, ref)
	require.ErrorIs(t, err, vivid.ErrorActorAlreadyExists)
	require.Zero(t, called.Load(), "同名冲突在预启动回调之前拒绝")
	system.EventStream().Publish(system, initializationEvent{})
	receiveRetryResult(t, events)
	require.NoError(t, system.Ask(live, "cancel", time.Second).Wait(), "重复创建不能清理存活 Actor 的调度任务")
	value, exists := system.actorContexts.Load(live.GetPath())
	require.True(t, exists)
	require.Same(t, ctx, value)
}

func TestContext_InitializationReservation(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	entered, release := make(chan vivid.ActorRef, 1), make(chan struct{})
	var once sync.Once
	resume := func() { once.Do(func() { close(release) }) }
	defer resume()
	result := make(chan error, 1)
	go func() {
		_, err := NewContext(system.System, system.Ref(), vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
			entered <- ctx.Ref()
			<-release
			return context.Canceled
		}), vivid.WithActorName("reserved"))
		result <- err
	}()
	ref := receiveRetryResult(t, entered)
	_, err := system.FindActor(ref.String())
	require.ErrorIs(t, err, vivid.ErrorNotFound, "预留名称不能暴露未初始化的邮箱")
	require.False(t, system.HeartbeatProbe(ref).Available)
	var called atomic.Int32
	_, err = NewContext(system.System, system.Ref(), vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
		called.Add(1)
		return nil
	}), vivid.WithActorName("reserved"))
	require.ErrorIs(t, err, vivid.ErrorActorAlreadyExists)
	require.Zero(t, called.Load())
	resume()
	require.ErrorIs(t, receiveRetryResult(t, result), context.Canceled)
	_, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}), vivid.WithActorName("reserved"))
	require.NoError(t, err)
}

type rollbackEventStream struct {
	vivid.EventStream
	entered chan struct{}
	release chan struct{}
}

func (s *rollbackEventStream) UnsubscribeAll(ctx vivid.EventStreamContext) {
	if ctx.Ref().GetPath() == "/rollback" {
		close(s.entered)
		<-s.release
	}
	s.EventStream.UnsubscribeAll(ctx)
}

func TestContext_InitializationReservesUntilRollbackEnds(t *testing.T) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	stream := &rollbackEventStream{EventStream: system.eventStream, entered: make(chan struct{}), release: make(chan struct{})}
	system.eventStream = stream
	require.NoError(t, system.Start())
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	var once sync.Once
	resume := func() { once.Do(func() { close(stream.release) }) }
	defer resume()
	result := make(chan error, 1)
	go func() {
		_, err := NewContext(system, system.Ref(), vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
			ctx.EventStream().Subscribe(ctx, initializationEvent{})
			return context.Canceled
		}), vivid.WithActorName("rollback"))
		result <- err
	}()
	receiveRetryResult(t, stream.entered)
	_, err := NewContext(system, system.Ref(), vivid.ActorFN(func(ctx vivid.ActorContext) {}), vivid.WithActorName("rollback"))
	require.ErrorIs(t, err, vivid.ErrorActorAlreadyExists, "旧创建尚在清理时不能将名称交给新实例")
	_, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}), vivid.WithActorName("independent"))
	require.NoError(t, err, "清理期间不能持创建名称锁阻塞无关 Actor")
	resume()
	require.ErrorIs(t, receiveRetryResult(t, result), context.Canceled)
}
