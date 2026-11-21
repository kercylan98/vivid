package actor

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestScheduler_CronReleasesRegistration(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	delivered := make(chan int, 2)
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(int); ok {
			delivered <- message
		}
	}))
	require.NoError(t, err)
	// 指定年月日与秒，只存在一个未来触发点，覆盖真实 Cron 的自然结束。
	at := time.Now().UTC().Add(2 * time.Second)
	expression := fmt.Sprintf("%d %d %d %d %d ? %d", at.Second(), at.Minute(), at.Hour(), at.Day(), at.Month(), at.Year())
	option := vivid.WithSchedulerReference("finite")
	task, err := system.Scheduler().Cron(target, expression, 1, option, vivid.WithScheduleLocation(time.UTC))
	require.NoError(t, err)
	require.Equal(t, 1, receiveRetryResult(t, delivered))
	require.Eventually(t, func() bool { return !task.Exists() }, time.Second, time.Millisecond)
	requireScheduledTaskReleased(t, task)
	require.False(t, system.Scheduler().Exists("finite"))
	next, err := system.Scheduler().Once(target, 0, 2, option)
	require.NoError(t, err, "结束的 Cron 不占用名称")
	require.Equal(t, 2, receiveRetryResult(t, delivered))
	requireScheduledTaskReleased(t, next)
}

type schedulerFiniteSchedule struct{ at time.Time }

func (s *schedulerFiniteSchedule) Next(after time.Time) time.Time {
	if s.at.After(after) {
		return s.at
	}
	return time.Time{}
}

func TestScheduler_CronExpiredRegistration(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler.Load()
	option := vivid.WithSchedulerReference("expired")
	for _, expression := range []string{"0 0 0 1 1 ? 2000", "0 0 0 31 2 ?"} {
		task, err := scheduler.Cron(system.Ref(), expression, 1, option)
		require.ErrorIs(t, err, vivid.ErrorScheduledTaskExpired)
		require.Nil(t, task)
		require.False(t, scheduler.Exists("expired"))
	}
	next, err := scheduler.Cron(system.Ref(), "0 0 0 1 1 ? 9999", 1, option)
	require.NoError(t, err, "远期年份不能因纳秒时间戳回绕而失败")
	require.True(t, next.Exists())
	require.NoError(t, next.Cancel())
}

func TestScheduler_CronLateCallback(t *testing.T) {
	for _, action := range []string{"cancel", "clear", "close"} {
		t.Run(action, func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			scheduler := system.Context.scheduler.Load()
			receiver := &onceObservedRef{ActorRef: system.Ref()}
			option := vivid.WithSchedulerReference("same")
			plan := &schedulerFiniteSchedule{at: time.Now().Add(time.Hour)}
			task, err := scheduler.scheduleCron(receiver, 1, vivid.NewScheduleOptions(option), plan)
			require.NoError(t, err)
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
			require.False(t, timer.Stop(), "取消、清理和关闭都停止计时器")
			requireScheduledTaskReleased(t, task)
			current := scheduler
			if action == "close" {
				current = newScheduler(system.Context)
				defer current.close()
			}
			for _, kind := range []string{"once", "loop", "cron"} {
				next, err := registerScheduledTask(current, system.Ref(), kind, option)
				require.NoError(t, err)
				scheduler.fireCron(old, receiver, 1, plan, time.Now().Add(-time.Second))
				require.Zero(t, receiver.called.Load(), "旧回调不能继续投递")
				require.True(t, next.Exists(), "旧回调不能删除同名新任务")
				require.NoError(t, next.Cancel())
			}
		})
	}
}

func TestScheduler_CronEarlyCallback(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler.Load()
	receiver := &onceObservedRef{ActorRef: system.Ref()}
	at := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	plan := &schedulerFiniteSchedule{at: at}
	task, err := scheduler.scheduleCron(receiver, 1, vivid.NewScheduleOptions(), plan)
	require.NoError(t, err)
	internal := task.(*scheduledTask)
	require.True(t, internal.timer.Stop())
	// 固定模拟超长等待的分段计时器提前唤醒，不实际等待数百年。
	scheduler.fireCron(internal, receiver, 1, plan, at)
	require.Zero(t, receiver.called.Load())
	require.True(t, task.Exists())
	require.True(t, internal.timer.Stop(), "未到日历时间应继续等待")
	require.NoError(t, task.Cancel())
}

func TestScheduler_CronExpiryPrecedesDelivery(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	delivered := make(chan int, 1)
	ready := make(chan *Context, 1)
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			ready <- ctx.(*Context)
		case int:
			delivered <- message
		}
	}))
	require.NoError(t, err)
	ctx := receiveRetryResult(t, ready)
	receiver := &onceClaimedRef{ActorRef: target, entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(receiver.release) }) }
	defer release()
	scheduler := ctx.scheduler.Load()
	option := vivid.WithSchedulerReference("in-flight")
	plan := &schedulerFiniteSchedule{at: time.Now().Add(20 * time.Millisecond)}
	task, err := scheduler.scheduleCron(receiver, 1, vivid.NewScheduleOptions(option), plan)
	require.NoError(t, err)
	receiveRetryResult(t, receiver.entered)
	requireScheduledTaskReleased(t, task)
	next, err := scheduler.Once(target, time.Hour, 2, option)
	require.NoError(t, err, "末次投递阻塞时即可复用名称")
	release()
	// 已被取出的末次投递仍可以完成，且不会撤销新登记。
	require.Equal(t, 1, receiveRetryResult(t, delivered))
	require.True(t, next.Exists())
	require.NoError(t, next.Cancel())
}
