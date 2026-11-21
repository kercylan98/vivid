package actor

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func registerScheduledTask(scheduler vivid.Scheduler, target vivid.ActorRef, kind string, options ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	switch kind {
	case "once":
		return scheduler.Once(target, time.Hour, 1, options...)
	case "loop":
		return scheduler.Loop(target, time.Hour, 1, options...)
	default:
		return scheduler.Cron(target, "0 0 0 1 1 ? 2099", 1, options...)
	}
}

func requireScheduledTaskReleased(t *testing.T, task vivid.ScheduledTask) {
	t.Helper()
	require.False(t, task.Exists())
	require.ErrorIs(t, task.Cancel(), vivid.ErrorNotFound)
	internal := task.(*scheduledTask)
	require.Nil(t, internal.owner.Load(), "已结束句柄不保留 Actor 上下文")
	require.Nil(t, internal.timer, "已结束句柄不通过计时器保留消息")
}

func TestScheduledTask_GeneratedReference(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	seen := make(map[string]bool)
	for _, kind := range []string{"once", "loop", "cron"} {
		task, err := registerScheduledTask(system.Scheduler(), system.Ref(), kind)
		require.NoError(t, err)
		require.NotNil(t, task)
		reference := task.Reference()
		require.NotEmpty(t, reference)
		require.False(t, seen[reference])
		seen[reference] = true
		require.True(t, task.Exists())
		require.True(t, system.Scheduler().Exists(reference))
		require.NoError(t, task.Cancel(), "无需自行生成名称即可取消")
		requireScheduledTaskReleased(t, task)
		require.Equal(t, reference, task.Reference(), "终结不改变注册名称")
		require.False(t, system.Scheduler().Exists(reference))
	}
}

func TestScheduledTask_Identity(t *testing.T) {
	for _, kind := range []string{"once", "loop", "cron"} {
		for _, action := range []string{"cancel", "name", "clear", "close"} {
			t.Run(kind+"/"+action, func(t *testing.T) {
				system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
				t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
				scheduler := system.Context.scheduler.Load()
				option := vivid.WithSchedulerReference("same")
				old, err := registerScheduledTask(scheduler, system.Ref(), kind, option)
				require.NoError(t, err)
				switch action {
				case "cancel":
					require.NoError(t, old.Cancel())
				case "name":
					require.NoError(t, scheduler.Cancel("same"))
				case "clear":
					scheduler.Clear()
				case "close":
					scheduler.close()
				}
				requireScheduledTaskReleased(t, old)
				current := scheduler
				if action == "close" {
					current = newScheduler(system.Context)
					defer current.close()
				}
				// 新任务逐一使用全部调度类型，句柄身份不依赖任务种类。
				for _, nextKind := range []string{"once", "loop", "cron"} {
					next, err := registerScheduledTask(current, system.Ref(), nextKind, option)
					require.NoError(t, err)
					require.Equal(t, old.Reference(), next.Reference())
					require.False(t, old.Exists())
					require.ErrorIs(t, old.Cancel(), vivid.ErrorNotFound)
					// 固定重放旧句柄读取 owner 后，被另一调用清理并同名重注册的窗口。
					require.ErrorIs(t, scheduler.cancel("same", old.(*scheduledTask)), vivid.ErrorNotFound)
					require.True(t, next.Exists(), "旧句柄不能取消同名新任务")
					require.NoError(t, next.Cancel())
				}
			})
		}
	}
}

func TestScheduledTask_ConcurrentCancel(t *testing.T) {
	for _, kind := range []string{"once", "loop", "cron"} {
		t.Run(kind, func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			task, err := registerScheduledTask(system.Scheduler(), system.Ref(), kind)
			require.NoError(t, err)
			start := make(chan struct{})
			var cancelled atomic.Int32
			var wg sync.WaitGroup
			for range 16 {
				wg.Go(func() {
					<-start
					_ = task.Exists()
					if err := task.Cancel(); err == nil {
						cancelled.Add(1)
					} else if !errors.Is(err, vivid.ErrorNotFound) {
						t.Error(err)
					}
				})
			}
			close(start)
			wg.Wait()
			require.EqualValues(t, 1, cancelled.Load())
			requireScheduledTaskReleased(t, task)
		})
	}
}

func TestScheduledTask_RegistrationFailure(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() {
		if !system.requestsStopped() {
			require.NoError(t, system.Stop(time.Second))
		}
	})
	scheduler := system.Context.scheduler.Load()
	for _, kind := range []string{"once", "loop", "cron"} {
		task, err := registerScheduledTask(scheduler, nil, kind)
		require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
		require.Nil(t, task)
		task, err = registerScheduledTask(scheduler, system.Ref(), kind, vivid.WithScheduleOptions(vivid.ScheduleOptions{}))
		require.NoError(t, err)
		require.NotEmpty(t, task.Reference())
		require.NoError(t, task.Cancel())
		current, err := registerScheduledTask(scheduler, system.Ref(), kind, vivid.WithSchedulerReference("same"))
		require.NoError(t, err)
		task, err = registerScheduledTask(scheduler, system.Ref(), kind, vivid.WithSchedulerReference("same"))
		require.ErrorIs(t, err, vivid.ErrorScheduledTaskAlreadyExists)
		require.Nil(t, task)
		require.True(t, current.Exists())
		require.NoError(t, current.Cancel())
	}
	task, err := scheduler.Cron(system.Ref(), "invalid", 1)
	require.ErrorIs(t, err, vivid.ErrorCronParse)
	require.Nil(t, task)
	scheduler.close()
	for _, kind := range []string{"once", "loop", "cron"} {
		task, err := registerScheduledTask(scheduler, system.Ref(), kind)
		require.ErrorIs(t, err, vivid.ErrorActorDeaded)
		require.Nil(t, task)
	}
	require.NoError(t, system.Stop(time.Second))
	for _, kind := range []string{"once", "loop", "cron"} {
		task, err := registerScheduledTask(scheduler, system.Ref(), kind)
		require.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
		require.Nil(t, task)
	}
}

func TestScheduledTask_ActorLifecycle(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	started := make(chan vivid.ScheduledTask, 2)
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
			task, err := ctx.Scheduler().Once(system.Ref(), time.Hour, 1, vivid.WithSchedulerReference("same"))
			if err != nil {
				panic(err)
			}
			started <- task
		}
	}))
	require.NoError(t, err)
	old := receiveRetryResult(t, started)
	system.AdvancedTell(true, owner, &RestartMessage{Reason: "task handle"})
	current := receiveRetryResult(t, started)
	requireScheduledTaskReleased(t, old)
	require.True(t, current.Exists())
	require.ErrorIs(t, old.Cancel(), vivid.ErrorNotFound)
	system.Kill(owner, false)
	require.Eventually(t, func() bool { return !current.Exists() }, time.Second, time.Millisecond)
	requireScheduledTaskReleased(t, current)
}

func TestScheduledTask_OptionsSnapshot(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	for _, kind := range []string{"once", "loop", "cron"} {
		var configured *vivid.ScheduleOptions
		task, err := registerScheduledTask(system.Scheduler(), system.Ref(), kind, func(opts *vivid.ScheduleOptions) {
			opts.Reference = "original"
			configured = opts
		})
		require.NoError(t, err)
		configured.Reference = "changed"
		require.Equal(t, "original", task.Reference(), "句柄不继续读取用户保存的配置对象")
		require.True(t, task.Exists())
		require.NoError(t, task.Cancel())
		requireScheduledTaskReleased(t, task)
	}
}
