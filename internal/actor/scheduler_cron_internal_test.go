package actor

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/reugn/go-quartz/quartz"
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

type schedulerFiniteTrigger struct {
	delay    time.Duration
	started  bool
	delivery bool
	next     int64
	entered  chan struct{}
	release  chan struct{}
}

func (t *schedulerFiniteTrigger) NextFireTime(prev int64) (int64, error) {
	// 在途测试只在 Quartz 接纳本次投递时结束，避免机器繁忙触发 misfire 而跳过观测点。
	if !t.started || t.delivery && prev != t.next {
		t.started = true
		t.next = prev + int64(t.delay)
		return t.next, nil
	}
	if t.entered != nil {
		close(t.entered)
		<-t.release
	}
	return 0, quartz.ErrTriggerExpired
}

func (t *schedulerFiniteTrigger) Description() string {
	return "finite"
}

func TestScheduler_CronMissedFinalFire(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler.Load()
	observed := &onceObservedRef{ActorRef: system.Ref()}
	// 末次时间已超出 Quartz 的过期窗口，不执行 job，仍需回收登记。
	trigger := &schedulerCronTrigger{Trigger: &schedulerFiniteTrigger{delay: -time.Second}}
	task, err := scheduler.scheduleJob(observed, 1, vivid.NewScheduleOptions(), trigger, "cron")
	require.NoError(t, err)
	require.Eventually(t, func() bool { return !task.Exists() }, time.Second, time.Millisecond)
	requireScheduledTaskReleased(t, task)
	require.Zero(t, observed.called.Load(), "错过的末次任务不能为回收登记而补投递")
	require.Error(t, system.scheduler.DeleteJob(scheduler.jobKey(task.Reference())))
}

func TestScheduler_CronExpiryAndCleanup(t *testing.T) {
	for _, action := range []string{"cancel", "clear", "close"} {
		t.Run(action, func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			scheduler := system.Context.scheduler.Load()
			finite := &schedulerFiniteTrigger{delay: -time.Second, entered: make(chan struct{}), release: make(chan struct{})}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(finite.release) }) }
			defer release()
			option := vivid.WithSchedulerReference("same")
			task, err := scheduler.scheduleJob(system.Ref(), 1, vivid.NewScheduleOptions(option), &schedulerCronTrigger{Trigger: finite}, "cron")
			require.NoError(t, err)
			receiveRetryResult(t, finite.entered)
			cleaned := make(chan error, 1)
			go func() {
				switch action {
				case "cancel":
					cleaned <- task.Cancel()
				case "clear":
					scheduler.Clear()
					cleaned <- nil
				case "close":
					scheduler.close()
					cleaned <- nil
				}
			}()
			// 固定登记锁 -> 队列锁的清理窗口，终结通知不能反向等待登记锁。
			require.Eventually(t, func() bool {
				if scheduler.mu.TryLock() {
					scheduler.mu.Unlock()
					return false
				}
				return true
			}, time.Second, time.Millisecond)
			release()
			require.NoError(t, receiveRetryResult(t, cleaned), "底层已自然移除时取消也应成功")
			requireScheduledTaskReleased(t, task)
			current := scheduler
			if action == "close" {
				current = newScheduler(system.Context)
				defer current.close()
			}
			for _, kind := range []string{"once", "loop", "cron"} {
				next, err := registerScheduledTask(current, system.Ref(), kind, option)
				require.NoError(t, err)
				// 重放未取得登记锁的旧终结通知，不能移除同名新登记或底层任务。
				scheduler.expireTask(task.(*scheduledTask))
				require.True(t, next.Exists())
				require.NoError(t, next.Cancel())
			}
		})
	}
}

func TestScheduler_CronExpiredRegistration(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler.Load()
	option := vivid.WithSchedulerReference("expired")
	task, err := scheduler.Cron(system.Ref(), "0 0 0 1 1 ? 2000", 1, option)
	require.ErrorIs(t, err, quartz.ErrTriggerExpired)
	require.Nil(t, task)
	require.False(t, scheduler.Exists("expired"))
	next, err := scheduler.Cron(system.Ref(), "0 0 0 1 1 ? 2099", 1, option)
	require.NoError(t, err)
	require.True(t, next.Exists())
	require.NoError(t, next.Cancel())
	trigger := &schedulerCronTrigger{Trigger: &quartz.RunOnceTrigger{Expired: true}}
	failed, err := scheduler.scheduleJob(system.Ref(), 1, vivid.NewScheduleOptions(option), trigger, "cron")
	require.ErrorIs(t, err, quartz.ErrTriggerExpired)
	require.Nil(t, failed)
	requireScheduledTaskReleased(t, trigger.task)
}

func TestScheduler_CronImmediateExpiry(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler.Load()
	for range 32 {
		// 末次触发可紧跟底层提交，回收必须等待本地登记发布。
		trigger := &schedulerCronTrigger{Trigger: &schedulerFiniteTrigger{delay: -time.Second}}
		task, err := scheduler.scheduleJob(system.Ref(), 1, vivid.NewScheduleOptions(vivid.WithSchedulerReference("immediate")), trigger, "cron")
		require.NoError(t, err)
		require.Eventually(t, func() bool { return !task.Exists() }, time.Second, time.Millisecond)
		requireScheduledTaskReleased(t, task)
	}
}

func TestScheduler_CronExpiryPrecedesDelivery(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	delivered := make(chan int, 1)
	ready := make(chan vivid.Scheduler, 1)
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			ready <- ctx.Scheduler()
		case int:
			delivered <- message
		}
	}))
	require.NoError(t, err)
	receiver := &onceClaimedRef{ActorRef: target, entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(receiver.release) }) }
	defer release()
	scheduler := receiveRetryResult(t, ready).(*Scheduler)
	option := vivid.WithSchedulerReference("in-flight")
	trigger := &schedulerCronTrigger{Trigger: &schedulerFiniteTrigger{delay: time.Millisecond, delivery: true}}
	task, err := scheduler.scheduleJob(receiver, 1, vivid.NewScheduleOptions(option), trigger, "cron")
	require.NoError(t, err)
	receiveRetryResult(t, receiver.entered)
	require.Eventually(t, func() bool { return !task.Exists() }, time.Second, time.Millisecond)
	requireScheduledTaskReleased(t, task)
	next, err := scheduler.Loop(target, time.Hour, 2, option)
	require.NoError(t, err)
	release()
	require.Equal(t, 1, receiveRetryResult(t, delivered), "回收不能撤回末次在途投递")
	require.True(t, next.Exists(), "末次消息投递完成不能清理同名新任务")
	require.NoError(t, next.Cancel())
}
