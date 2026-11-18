package actor

import (
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

type schedulerCleanupLogger struct {
	log.Logger
	done chan struct{}
	once sync.Once
}

func (l *schedulerCleanupLogger) Debug(message string, args ...any) {
	if message == "actor killed" {
		l.once.Do(func() { close(l.done) })
	}
	l.Logger.Debug(message, args...)
}

func TestScheduler_OldCleanupPreservesReplacement(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "expired"}[expired], func(t *testing.T) {
			logger := log.NewTextLogger(log.WithLevel(log.LevelError))
			oldLogger := &schedulerCleanupLogger{Logger: logger, done: make(chan struct{})}
			system := NewSystem(vivid.WithActorSystemLogger(logger))
			paused, release := make(chan struct{}), make(chan struct{})
			var releaseOnce, pauseOnce sync.Once
			resume := func() { releaseOnce.Do(func() { close(release) }) }
			system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
				if event, ok := event.(ves.ActorKilledEvent); ok && event.ActorRef.GetPath() == "/reuse" {
					pauseOnce.Do(func() {
						close(paused)
						<-release
					})
				}
			}}
			require.NoError(t, system.Start())
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			defer resume()
			ticks := make(chan struct{}, 1)
			target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(int); ok {
					ticks <- struct{}{}
				}
			}))
			require.NoError(t, err)
			started := make(chan error, 2)
			var generation atomic.Int32
			behavior := vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case *vivid.OnLaunch:
					delay := time.Hour
					if generation.Add(1) == 1 && expired {
						delay = time.Millisecond
					}
					started <- schedulerResultError(ctx.Scheduler().Once(target, delay, 1, vivid.WithSchedulerReference("same")))
				case string:
					ctx.Reply(ctx.Scheduler().Cancel("same"))
				}
			})
			old, err := system.ActorOf(behavior, vivid.WithActorName("reuse"), vivid.WithActorLogger(oldLogger))
			require.NoError(t, err)
			require.NoError(t, receiveRetryResult(t, started))
			if expired {
				receiveRetryResult(t, ticks)
			}
			system.Kill(old, false)
			receiveRetryResult(t, paused)
			// 根 Actor 已收到旧实例的子级死亡通知，再创建同名实例。
			_, err = system.Ping(system.Ref(), time.Second)
			require.NoError(t, err)
			current, err := system.ActorOf(behavior, vivid.WithActorName("reuse"))
			require.NoError(t, err)
			require.NoError(t, receiveRetryResult(t, started), "新实例不能与旧任务的 reference 冲突")
			resume()
			receiveRetryResult(t, oldLogger.done)
			require.NoError(t, system.Ask(current, "cancel", time.Second).Wait(), "旧实例清理不能删除新实例的任务")
		})
	}
}

func TestScheduler_ReferencesBelongToOwner(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	type request struct {
		reference string
		clear     bool
	}
	started := make(chan error, 2)
	actor := vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			reference := "part:timer"
			if ctx.Ref().GetPath() == "/owner:part" {
				reference = "timer"
			}
			started <- schedulerResultError(ctx.Scheduler().Once(ctx.Ref(), time.Hour, 1, vivid.WithSchedulerReference(reference)))
		case request:
			if message.clear {
				ctx.Scheduler().Clear()
				ctx.Reply(nil)
			} else {
				ctx.Reply(ctx.Scheduler().Cancel(message.reference))
			}
		}
	})
	first, err := system.ActorOf(actor, vivid.WithActorName("owner:part"))
	require.NoError(t, err)
	second, err := system.ActorOf(actor, vivid.WithActorName("owner"))
	require.NoError(t, err)
	require.NoError(t, receiveRetryResult(t, started))
	require.NoError(t, receiveRetryResult(t, started), "路径和 reference 包含分隔符时仍须区分所有者")
	require.NoError(t, system.Ask(first, request{clear: true}, time.Second).Wait())
	require.NoError(t, system.Ask(second, request{reference: "part:timer"}, time.Second).Wait())
}

func TestScheduler_RestartRegistersSameReference(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	started := make(chan *Context, 2)
	ticks := make(chan int, 2)
	var generation int
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			generation++
			if err := schedulerResultError(ctx.Scheduler().Once(ctx.Ref(), time.Millisecond, generation, vivid.WithSchedulerReference("tick"))); err != nil {
				panic(err)
			}
			started <- ctx.(*Context)
		case int:
			ticks <- message
		}
	}))
	require.NoError(t, err)
	ctx := receiveRetryResult(t, started)
	require.Equal(t, 1, receiveRetryResult(t, ticks))
	ctx.mailbox.Enqueue(mailbox.NewEnvelop(true, owner, owner, &RestartMessage{Reason: "scheduler owner"}))
	require.Same(t, ctx, receiveRetryResult(t, started))
	require.Equal(t, 2, receiveRetryResult(t, ticks), "重启清理后仍可用相同 reference 注册并实际触发")
}
