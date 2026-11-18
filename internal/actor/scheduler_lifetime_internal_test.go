package actor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/require"
)

func requireSchedulerRejected(t *testing.T, scheduler vivid.Scheduler, target vivid.ActorRef, reason error) {
	t.Helper()
	option := vivid.WithSchedulerReference("rejected")
	require.ErrorIs(t, schedulerResultError(scheduler.Once(target, time.Hour, 1, option)), reason)
	require.ErrorIs(t, schedulerResultError(scheduler.Loop(target, time.Hour, 1, option)), reason)
	require.ErrorIs(t, schedulerResultError(scheduler.Cron(target, "0 0 0 * * ?", 1, option)), reason)
	scheduler.Clear()
	require.False(t, scheduler.Exists("rejected"), "Clear 不能重新开放已关闭的调度器")
}

func TestScheduler_LifetimeTermination(t *testing.T) {
	logger := log.NewTextLogger(log.WithLevel(log.LevelError))
	ownerLogger := &schedulerCleanupLogger{Logger: logger, done: make(chan struct{})}
	system := NewTestSystem(t, vivid.WithActorSystemLogger(logger))
	t.Cleanup(func() {
		if !system.requestsStopped() {
			require.NoError(t, system.Stop(time.Second))
		}
	})
	saved := make(chan vivid.Scheduler, 1)
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
			saved <- ctx.Scheduler()
		}
	}), vivid.WithActorLogger(ownerLogger))
	require.NoError(t, err)
	scheduler := receiveRetryResult(t, saved)
	require.NoError(t, schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 1, vivid.WithSchedulerReference("pending"))))
	system.Kill(owner, false)
	receiveRetryResult(t, ownerLogger.done)
	require.False(t, scheduler.Exists("pending"))
	requireSchedulerRejected(t, scheduler, system.Ref(), vivid.ErrorActorDeaded)
	require.NoError(t, system.Stop(time.Second))
	requireSchedulerRejected(t, scheduler, system.Ref(), vivid.ErrorActorSystemStopped)
	requireSchedulerRejected(t, system.Scheduler(), system.Ref(), vivid.ErrorActorSystemStopped)
}

func TestScheduler_LifetimeRestart(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	saved := make(chan vivid.Scheduler, 2)
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
			saved <- ctx.Scheduler()
		}
	}))
	require.NoError(t, err)
	old := receiveRetryResult(t, saved)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	resume := func() { once.Do(func() { close(release) }) }
	defer resume()
	registered := make(chan error, 1)
	go func() {
		registered <- schedulerResultError(old.Once(system.Ref(), time.Hour, 1, func(opts *vivid.ScheduleOptions) {
			opts.Reference = "same"
			close(entered)
			<-release
		}))
	}()
	receiveRetryResult(t, entered)
	system.AdvancedTell(true, owner, &RestartMessage{Reason: "scheduler lifetime"})
	current := receiveRetryResult(t, saved)
	require.NotSame(t, old, current, "重启使用独立的调度器，旧句柄不重新开放")
	require.NoError(t, schedulerResultError(current.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("same"))))
	resume()
	require.ErrorIs(t, receiveRetryResult(t, registered), vivid.ErrorActorDeaded, "重启前开始的注册不能落入新实例")
	requireSchedulerRejected(t, old, system.Ref(), vivid.ErrorActorDeaded)
	require.ErrorIs(t, old.Cancel("same"), vivid.ErrorNotFound)
	old.Clear()
	require.NoError(t, current.Cancel("same"), "旧句柄清理不能影响新实例任务")
}

func TestScheduler_LifetimeInitializationFailure(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	var scheduler vivid.Scheduler
	_, err := system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
		scheduler = ctx.(*Context).Scheduler()
		return context.Canceled
	}, vivid.ActorFN(func(ctx vivid.ActorContext) {})))
	require.ErrorIs(t, err, context.Canceled)
	requireSchedulerRejected(t, scheduler, system.Ref(), vivid.ErrorActorDeaded)
}

func TestScheduler_LifetimeCloseDuringRegistration(t *testing.T) {
	for _, committing := range []bool{false, true} {
		t.Run(map[bool]string{false: "options", true: "committing"}[committing], func(t *testing.T) {
			logger := log.NewTextLogger(log.WithLevel(log.LevelError))
			ownerLogger := &schedulerCleanupLogger{Logger: logger, done: make(chan struct{})}
			system := NewTestSystem(t, vivid.WithActorSystemLogger(logger))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			saved := make(chan vivid.Scheduler, 1)
			killed := make(chan struct{}, 1)
			owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch message := ctx.Message().(type) {
				case *vivid.OnLaunch:
					saved <- ctx.Scheduler()
				case *vivid.OnKilled:
					if message.Ref.Equals(ctx.Ref()) {
						killed <- struct{}{}
					}
				}
			}), vivid.WithActorLogger(ownerLogger))
			require.NoError(t, err)
			scheduler := receiveRetryResult(t, saved).(*Scheduler)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			resume := func() { once.Do(func() { close(release) }) }
			defer resume()
			result := make(chan error, 1)
			go func() {
				if committing {
					trigger := &schedulerRegistrationTrigger{Trigger: quartz.NewRunOnceTrigger(time.Hour), entered: entered, release: release}
					result <- schedulerResultError(scheduler.scheduleJob(system.Ref(), 1, vivid.NewScheduleOptions(vivid.WithSchedulerReference("pending")), trigger, "once"))
				} else {
					result <- schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 1, func(opts *vivid.ScheduleOptions) {
						opts.Reference = "pending"
						close(entered)
						<-release
					}))
				}
			}()
			receiveRetryResult(t, entered)
			system.Kill(owner, false)
			receiveRetryResult(t, killed)
			if committing {
				resume()
				require.NoError(t, receiveRetryResult(t, result))
			} else {
				receiveRetryResult(t, ownerLogger.done)
				resume()
				require.ErrorIs(t, receiveRetryResult(t, result), vivid.ErrorActorDeaded)
			}
			receiveRetryResult(t, ownerLogger.done)
			require.False(t, scheduler.Exists("pending"))
			require.Error(t, system.scheduler.DeleteJob(scheduler.jobKey("pending")), "关闭不能漏掉先取得登记锁的底层任务")
		})
	}
}

func TestScheduler_LifetimeStopTimeout(t *testing.T) {
	logger := log.NewTextLogger(log.WithLevel(log.LevelError))
	ownerLogger := &schedulerCleanupLogger{Logger: logger, done: make(chan struct{})}
	system := NewTestSystem(t, vivid.WithActorSystemLogger(logger))
	t.Cleanup(func() {
		if !system.requestsStopped() {
			require.NoError(t, system.Stop(time.Second))
		}
	})
	saved := make(chan vivid.Scheduler, 1)
	closing := make(chan error, 1)
	final := make(chan error, 1)
	release := make(chan struct{})
	var once sync.Once
	resume := func() { once.Do(func() { close(release) }) }
	defer resume()
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			saved <- ctx.Scheduler()
		case *vivid.OnKill:
			closing <- schedulerResultError(ctx.Scheduler().Once(system.Ref(), time.Hour, 1, vivid.WithSchedulerReference("cleanup")))
			<-release
		case *vivid.OnKilled:
			if message.Ref.Equals(ctx.Ref()) {
				final <- schedulerResultError(ctx.Scheduler().Once(system.Ref(), time.Hour, 3, vivid.WithSchedulerReference("final")))
			}
		}
	}), vivid.WithActorLogger(ownerLogger))
	require.NoError(t, err)
	scheduler := receiveRetryResult(t, saved)
	result := make(chan error, 1)
	go func() { result <- system.Stop(time.Millisecond) }()
	require.NoError(t, receiveRetryResult(t, closing), "OnKill 中仍可安排收尾任务")
	require.Error(t, receiveRetryResult(t, result))
	require.NoError(t, schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("waiting"))), "Stop 等待超时不提前关闭仍在收尾的 Actor")
	resume()
	require.NoError(t, receiveRetryResult(t, final), "自身 OnKilled 中仍可安排收尾任务")
	receiveRetryResult(t, ownerLogger.done)
	if err := system.Stop(time.Second); err != nil {
		require.ErrorIs(t, err, vivid.ErrorActorSystemAlreadyStopped)
	}
	requireSchedulerRejected(t, scheduler, system.Ref(), vivid.ErrorActorSystemStopped)
	require.False(t, scheduler.Exists("cleanup"))
	require.False(t, scheduler.Exists("waiting"))
	require.False(t, scheduler.Exists("final"))
}

func TestScheduler_LifetimeRestartInitialization(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failed], func(t *testing.T) {
			logger := log.NewTextLogger(log.WithLevel(log.LevelError))
			system := NewTestSystem(t, vivid.WithActorSystemLogger(logger))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			saved := make(chan vivid.Scheduler, 1)
			pending := make(chan vivid.Scheduler, 1)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			resume := func() { once.Do(func() { close(release) }) }
			defer resume()
			owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
					saved <- ctx.Scheduler()
				}
			}), vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor {
				return vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
					scheduler := ctx.(*Context).Scheduler()
					if err := schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 1, vivid.WithSchedulerReference("initializing"))); err != nil {
						return err
					}
					pending <- scheduler
					close(entered)
					<-release
					if failed {
						return context.Canceled
					}
					return nil
				}, vivid.ActorFN(func(ctx vivid.ActorContext) {
					if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
						saved <- ctx.Scheduler()
					}
				}))
			})))
			require.NoError(t, err)
			old := receiveRetryResult(t, saved)
			system.AdvancedTell(true, owner, &RestartMessage{Reason: "scheduler initialization"})
			receiveRetryResult(t, entered)
			current := receiveRetryResult(t, pending)
			requireSchedulerRejected(t, old, system.Ref(), vivid.ErrorActorDeaded)
			resume()
			if failed {
				require.Eventually(t, func() bool { return !current.Exists("initializing") }, time.Second, time.Millisecond)
				requireSchedulerRejected(t, current, system.Ref(), vivid.ErrorActorDeaded)
				require.False(t, current.Exists("initializing"))
			} else {
				require.Same(t, current, receiveRetryResult(t, saved))
				require.NoError(t, current.Cancel("initializing"))
			}
		})
	}
}
