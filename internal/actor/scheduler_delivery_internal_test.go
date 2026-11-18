package actor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestScheduler_RejectsNilReceiver(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	var receiver *Ref
	for _, kind := range []string{"once", "loop", "cron"} {
		task, err := registerScheduledTask(system.Scheduler(), receiver, kind)
		require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
		require.Nil(t, task)
	}
	scheduler := system.Context.scheduler.Load()
	scheduler.close()
	task, err := scheduler.Cron(system.Ref(), "invalid", 1)
	require.ErrorIs(t, err, vivid.ErrorActorDeaded, "已关闭的调度器优先返回生命周期错误")
	require.Nil(t, task)
}

func TestScheduler_MaximumDuration(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	receiver := &onceObservedRef{ActorRef: system.Ref()}
	for _, kind := range []string{"once", "loop"} {
		var task vivid.ScheduledTask
		var err error
		if kind == "once" {
			task, err = system.Scheduler().Once(receiver, time.Duration(1<<63-1), 1)
		} else {
			task, err = system.Scheduler().Loop(receiver, time.Duration(1<<63-1), 1)
		}
		require.NoError(t, err)
		timer := task.(*scheduledTask).timer
		require.True(t, timer.Stop(), "最大正时长保持未来等待，不因溢出而空转")
		require.NoError(t, task.Cancel())
		requireScheduledTaskReleased(t, task)
	}
	require.Zero(t, receiver.called.Load())
}

type schedulerBlockingRef struct {
	vivid.ActorRef
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
	attempt chan time.Time
}

func (r *schedulerBlockingRef) Equals(other vivid.ActorRef) bool {
	if r.calls.Add(1) == 1 {
		close(r.entered)
		<-r.release
	}
	r.attempt <- time.Now()
	return r.ActorRef.Equals(other)
}

func TestScheduler_RecurringDelivery(t *testing.T) {
	for _, kind := range []string{"loop", "cron"} {
		for _, action := range []string{"continue", "cancel", "clear", "close"} {
			t.Run(kind+"/"+action, func(t *testing.T) {
				system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
				t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
				receiver := &schedulerBlockingRef{ActorRef: system.Ref(), entered: make(chan struct{}), release: make(chan struct{}), attempt: make(chan time.Time, 16)}
				var releaseOnce sync.Once
				release := func() { releaseOnce.Do(func() { close(receiver.release) }) }
				defer release()
				scheduler := system.Context.scheduler.Load()
				const interval = 5 * time.Millisecond
				option := vivid.WithSchedulerReference("same")
				var task vivid.ScheduledTask
				var err error
				plan := &schedulerDelayedSchedule{delay: interval}
				if kind == "loop" {
					task, err = scheduler.Loop(receiver, interval, 1, option)
				} else {
					task, err = scheduler.scheduleCron(receiver, 1, vivid.NewScheduleOptions(option), plan)
				}
				require.NoError(t, err)
				receiveRetryResult(t, receiver.entered)
				// 另一个真实计时任务完成，证明阻塞超过多个周期时调度器仍可推进其他任务。
				barrier := make(chan struct{}, 1)
				control, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					if _, ok := ctx.Message().(string); ok {
						barrier <- struct{}{}
					}
				}))
				require.NoError(t, err)
				_, err = scheduler.Once(control, 4*interval, "barrier")
				require.NoError(t, err)
				receiveRetryResult(t, barrier)
				require.EqualValues(t, 1, receiver.calls.Load(), "慢投递不能并发堆积回调")
				old := task.(*scheduledTask)
				timer := old.timer
				switch action {
				case "cancel":
					require.NoError(t, task.Cancel())
				case "clear":
					scheduler.Clear()
				case "close":
					scheduler.close()
				}
				release()
				first := receiveRetryResult(t, receiver.attempt)
				if action == "continue" {
					second := receiveRetryResult(t, receiver.attempt)
					if kind == "loop" {
						require.GreaterOrEqual(t, second.Sub(first), interval, "投递结束后才开始下个等待周期")
					}
					require.NoError(t, task.Cancel())
				} else {
					requireScheduledTaskReleased(t, task)
					current := scheduler
					if action == "close" {
						current = newScheduler(system.Context)
						defer current.close()
					}
					next, err := current.Once(control, time.Hour, "next", option)
					require.NoError(t, err)
					if kind == "loop" {
						scheduler.fireLoop(old, receiver, 1, interval)
					} else {
						scheduler.fireCron(old, receiver, 1, plan, time.Now().Add(-time.Second))
					}
					require.True(t, next.Exists(), "旧任务完成或迟到回调不能重置同名新任务")
					require.NoError(t, next.Cancel())
					require.False(t, timer.Stop(), "回收必须停止原计时器")
					require.EqualValues(t, 1, receiver.calls.Load())
				}
			})
		}
	}
}

func TestScheduler_FrameworkMessage(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	processed := make(chan bool, 1)
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *messages.PingMessage:
			processed <- false
		case string:
			processed <- true
		}
	}))
	require.NoError(t, err)
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			_, _ = ctx.Scheduler().Once(target, 0, &messages.PingMessage{})
		case *messages.PongMessage:
			ctx.Tell(target, "framework handled ping")
		}
	}))
	require.NoError(t, err)
	require.NotNil(t, owner)
	require.True(t, receiveRetryResult(t, processed), "调度框架消息也必须进入普通分发流程")
}

func TestScheduler_VirtualDelivery(t *testing.T) {
	system := NewTestSystem(t,
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("timer", vivid.ActorProviderFN(func() vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(int); ok {
					ctx.Reply(ctx.Sender())
				}
			})
		})),
	)
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	result := make(chan vivid.ActorRef, 1)
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			_, _ = ctx.Scheduler().Once(system.VirtualRef("timer", "one"), 0, 1)
		case vivid.ActorRef:
			result <- message
		}
	}))
	require.NoError(t, err)
	require.True(t, owner.Equals(receiveRetryResult(t, result)), "虚拟目标保留调度所有者作为发送方")
}

func TestScheduler_StashPreservesMessage(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	result := make(chan bool, 1)
	var stashed bool
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			_, _ = ctx.Scheduler().Once(ctx.Ref(), 0, 42)
		case int:
			if !stashed {
				stashed = true
				ctx.Stash()
				_, _ = ctx.Scheduler().Once(ctx.Ref(), 0, "ready")
			} else {
				result <- message == 42 && ctx.Ref().Equals(ctx.Sender())
			}
		case string:
			ctx.Unstash()
		}
	}))
	require.NoError(t, err)
	require.True(t, receiveRetryResult(t, result))
}

func TestScheduler_ParentCancellationKeepsCleanupTimers(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	system := NewTestSystem(t, vivid.WithActorSystemContext(parent), vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	release := make(chan struct{})
	close(release)
	observer := &schedulerBlockingRef{ActorRef: system.Ref(), entered: make(chan struct{}), release: release, attempt: make(chan time.Time, 16)}
	tasks := make(chan vivid.ScheduledTask, 1)
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnKill); ok {
			task, err := ctx.Scheduler().Loop(observer, time.Millisecond, 1)
			if err != nil {
				tasks <- nil
				return
			}
			tasks <- task
			<-observer.entered
		}
	}))
	require.NoError(t, err)
	cancel()
	task := receiveRetryResult(t, tasks)
	require.NotNil(t, task)
	receiveRetryResult(t, observer.entered)
	receiveRetryResult(t, system.stopCompleted)
	requireScheduledTaskReleased(t, task)
}

func TestScheduler_RecurringDeliveryPanic(t *testing.T) {
	for _, kind := range []string{"loop", "cron"} {
		t.Run(kind, func(t *testing.T) {
			logger := &onceFailureLogger{Logger: log.NewTextLogger(log.WithLevel(log.LevelError)), failed: make(chan struct{}, 16)}
			system := NewTestSystem(t, vivid.WithActorSystemLogger(logger))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			scheduler := system.Context.scheduler.Load()
			receiver := &onceObservedRef{ActorRef: system.Ref(), panic: true}
			var task vivid.ScheduledTask
			var err error
			if kind == "loop" {
				task, err = scheduler.Loop(receiver, time.Millisecond, 1)
			} else {
				task, err = scheduler.scheduleCron(receiver, 1, vivid.NewScheduleOptions(), &schedulerDelayedSchedule{delay: time.Millisecond})
			}
			require.NoError(t, err)
			for range 2 {
				receiveRetryResult(t, logger.failed)
			}
			require.True(t, task.Exists(), "单次投递 panic 不会留下永远不再触发的登记")
			require.NoError(t, task.Cancel())
			requireScheduledTaskReleased(t, task)
		})
	}
}
