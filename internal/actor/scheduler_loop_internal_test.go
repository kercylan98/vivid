package actor

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestScheduler_LoopRejectsNonPositiveInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Nanosecond, -time.Second, -1 << 63} {
		t.Run(interval.String(), func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			scheduler := system.Context.scheduler.Load()
			option := vivid.WithSchedulerReference("invalid")
			task, err := scheduler.Loop(system.Ref(), interval, 1, option)
			// 修改前也及时撤销已被错误接纳的任务，避免回归验证继续空转。
			if task != nil {
				_ = task.Cancel()
			}
			require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
			require.Nil(t, task)
			require.False(t, scheduler.Exists("invalid"))
			require.Error(t, system.scheduler.DeleteJob(scheduler.jobKey("invalid")), "非法周期不能进入底层队列")
			current, err := scheduler.Loop(system.Ref(), time.Hour, 1, option)
			require.NoError(t, err, "失败注册不占用名称")
			require.True(t, current.Exists())
			require.NoError(t, current.Cancel())
		})
	}
}

func TestScheduler_LoopAfterRejectedRegistration(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	ticks := make(chan struct{}, 16)
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(int); ok {
			select {
			case ticks <- struct{}{}:
			default:
			}
		}
	}))
	require.NoError(t, err)
	task, err := system.Scheduler().Loop(target, 5*time.Millisecond, 1, vivid.WithSchedulerReference("valid"))
	require.NoError(t, err)
	for range 32 {
		rejected, err := system.Scheduler().Loop(target, 0, 2)
		if rejected != nil {
			_ = rejected.Cancel()
		}
		require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
		require.Nil(t, rejected)
	}
	for range 3 {
		receiveRetryResult(t, ticks)
	}
	require.True(t, task.Exists(), "非法注册不能干扰已存在的周期任务")
	require.NoError(t, task.Cancel())
	require.False(t, task.Exists())
}

func TestScheduler_LoopInvalidIntervalPreservesCurrentTask(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	option := vivid.WithSchedulerReference("same")
	task, err := system.Scheduler().Loop(system.Ref(), time.Hour, 1, option)
	require.NoError(t, err)
	for _, interval := range []time.Duration{0, -time.Second} {
		rejected, err := system.Scheduler().Loop(system.Ref(), interval, 2, option)
		require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
		require.Nil(t, rejected)
		require.True(t, task.Exists(), "非法重注册不能替换同名存活任务")
	}
	require.NoError(t, task.Cancel())
}

func TestScheduler_LoopInvalidIntervalAfterClose(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() {
		if !system.requestsStopped() {
			require.NoError(t, system.Stop(time.Second))
		}
	})
	scheduler := system.Context.scheduler.Load()
	scheduler.close()
	for _, interval := range []time.Duration{0, -time.Second} {
		task, err := scheduler.Loop(system.Ref(), interval, 1)
		require.ErrorIs(t, err, vivid.ErrorActorDeaded)
		require.Nil(t, task)
	}
	require.NoError(t, system.Stop(time.Second))
	for _, interval := range []time.Duration{0, -time.Second} {
		task, err := scheduler.Loop(system.Ref(), interval, 1)
		require.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
		require.Nil(t, task)
	}
}
