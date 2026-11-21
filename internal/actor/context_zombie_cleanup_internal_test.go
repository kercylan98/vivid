package actor

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func TestContext_ZombieCleanupPreservesReplacement(t *testing.T) {
	for _, notification := range []string{"kill", "cached kill", "killed", "cleanup"} {
		t.Run(notification, func(t *testing.T) {
			system := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{}))
			t.Cleanup(func() { _ = system.Stop(time.Second) })
			var old vivid.ActorRef
			var cached vivid.ActorRef
			var deaths atomic.Int32
			processed := make(chan struct{}, 8)
			system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
				switch event := event.(type) {
				case ves.ActorKilledEvent:
					if event.ActorRef == old {
						deaths.Add(1)
						processed <- struct{}{}
					}
				case ves.DeathLetterEvent:
					if event.Envelope.Receiver() == old || event.Envelope.Receiver() == cached {
						processed <- struct{}{}
					}
				}
			}}
			require.NoError(t, system.Start())
			ready := make(chan *Context, 1)
			var err error
			old, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
					ready <- ctx.(*Context)
				}
			}), vivid.WithActorName("reuse"), vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { return nil })))
			require.NoError(t, err)
			owner := receiveRetryResult(t, ready)
			cached, err = system.ParseRef(old.String())
			require.NoError(t, err)
			_, err = system.Ping(cached, time.Second)
			require.NoError(t, err)
			system.AdvancedTell(true, old, &RestartMessage{Reason: "cleanup identity"})
			_, err = system.Ping(old, time.Second)
			require.NoError(t, err)
			require.True(t, owner.zombie)
			system.Kill(old, false)
			receiveRetryResult(t, processed)
			_, err = system.Ping(system.Ref(), time.Second)
			require.NoError(t, err)

			target, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
			require.NoError(t, err)
			var pending vivid.Future[vivid.Message]
			var task vivid.ScheduledTask
			replacement, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
					pending = ctx.Ask(target, "pending", 0)
					var scheduleErr error
					task, scheduleErr = ctx.Scheduler().Once(target, time.Hour, "pending")
					if scheduleErr != nil {
						panic(scheduleErr)
					}
					ctx.EventStream().Subscribe(ctx, 0)
					ready <- ctx.(*Context)
				}
			}), vivid.WithActorName("reuse"))
			require.NoError(t, err)
			current := receiveRetryResult(t, ready)
			_, err = system.Ping(replacement, time.Second)
			require.NoError(t, err)
			for range 3 {
				switch notification {
				case "kill":
					system.Kill(old, false)
					receiveRetryResult(t, processed)
				case "cached kill":
					system.Kill(cached, false)
					receiveRetryResult(t, processed)
				case "killed":
					system.AdvancedTell(true, old, &vivid.OnKilled{Ref: old})
					receiveRetryResult(t, processed)
				case "cleanup":
					system.closeContextRequests(owner)
					require.False(t, pending.(*future.Future[vivid.Message]).IsCompleted(), "已结束的旧生命周期不能清理新请求")
					system.removeActorContext(owner)
				}
				found, err := system.FindActor(replacement.String())
				require.NoError(t, err, "旧实例的迟到清理不能删除新实例路由")
				require.Same(t, replacement, found)
				require.False(t, pending.(*future.Future[vivid.Message]).IsCompleted(), "旧实例不能关闭新实例的请求")
				require.True(t, task.Exists())
				requireEventStreamSubscriptions(t, system.eventStream.(*startupEventStream).EventStream.(*eventStream), replacement, 0)
				_, closing := system.closingActors.Load(replacement)
				require.False(t, closing, "旧引用不能为新实例留下关闭标记")
			}
			require.EqualValues(t, 1, deaths.Load(), "最终清理和死亡事件只能执行一次")
			require.NoError(t, system.Stop(time.Second))
			require.ErrorIs(t, pending.Wait(), vivid.ErrorActorDeaded)
			require.False(t, task.Exists())
			requireEventStreamRejected(t, system.EventStream(), current)
			system.closingActors.Range(func(_, _ any) bool { t.Error("停止后不能保留关闭标记"); return true })
		})
	}
}

func TestContext_StaleCleanupPreservesClosingMarker(t *testing.T) {
	system := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{}))
	require.NoError(t, system.Start())
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	ready := make(chan *Context, 1)
	old, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
			ready <- ctx.(*Context)
		}
	}), vivid.WithActorName("reuse"))
	require.NoError(t, err)
	owner := receiveRetryResult(t, ready)
	system.Kill(old, false)
	require.Eventually(t, func() bool {
		_, err := system.FindActor(old.String())
		return err != nil
	}, time.Second, time.Millisecond)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	replacement, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnKill); ok {
			close(entered)
			<-release
		}
	}), vivid.WithActorName("reuse"))
	require.NoError(t, err)
	parsed, err := system.ParseRef(replacement.String())
	require.NoError(t, err)
	system.Kill(parsed, false)
	receiveRetryResult(t, entered)
	require.False(t, system.HeartbeatProbe(replacement).Available)
	system.unmarkActorAsClosing(old)
	require.False(t, system.HeartbeatProbe(replacement).Available, "旧实例不能删除新实例的关闭标记")
	system.removeActorContext(owner)
	_, found := system.actorContexts.Load(replacement.GetPath())
	require.True(t, found)
	once.Do(func() { close(release) })
}

func TestContext_ZombieIgnoresUnrelatedDeath(t *testing.T) {
	system := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{}))
	require.NoError(t, system.Start())
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	ref, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}),
		vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { return nil })))
	require.NoError(t, err)
	system.AdvancedTell(true, ref, &RestartMessage{Reason: "unrelated death"})
	_, err = system.Ping(ref, time.Second)
	require.NoError(t, err)
	unknown, err := system.CreateRef(ref.GetAddress(), ref.GetPath()+"/unknown")
	require.NoError(t, err)
	system.AdvancedTell(true, ref, &vivid.OnKilled{Ref: unknown})
	_, err = system.Ping(ref, time.Second)
	require.NoError(t, err)
	found, err := system.FindActor(ref.String())
	require.NoError(t, err, "无关死亡通知不能代替显式终止")
	require.Same(t, ref, found)
	require.False(t, system.HeartbeatProbe(ref).Available)
	require.NoError(t, system.Stop(time.Second))
}

func BenchmarkContext_CreateAndKill(b *testing.B) {
	for _, zombie := range []bool{false, true} {
		b.Run(map[bool]string{false: "normal", true: "zombie"}[zombie], func(b *testing.B) {
			system := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{}))
			finished := make(chan struct{}, 1)
			system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
				if event, ok := event.(ves.ActorKilledEvent); ok && event.ActorRef.GetPath() == "/worker" {
					finished <- struct{}{}
				}
			}}
			if err := system.Start(); err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := system.Stop(time.Second); err != nil {
					b.Error(err)
				}
			})
			options := []vivid.ActorOption{vivid.WithActorName("worker")}
			if zombie {
				options = append(options, vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { return nil })))
			}
			behavior := vivid.ActorFN(func(vivid.ActorContext) {})
			b.ReportAllocs()
			for b.Loop() {
				ref, err := system.ActorOf(behavior, options...)
				if err != nil {
					b.Fatal(err)
				}
				if zombie {
					system.AdvancedTell(true, ref, &RestartMessage{Reason: "cleanup benchmark"})
					if _, err := system.Ping(ref, time.Second); err != nil {
						b.Fatal(err)
					}
				}
				system.Kill(ref, false)
				<-finished
			}
		})
	}
}
