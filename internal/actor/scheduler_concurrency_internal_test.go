package actor

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

func TestScheduler_ConcurrentRegistration(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler.Load()
	const workers, rounds = 16, 100
	start := make(chan struct{})
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			<-start
			for round := range rounds {
				reference := strconv.Itoa(worker) + "/" + strconv.Itoa(round)
				if err := schedulerResultError(scheduler.Loop(system.Ref(), time.Hour, round, vivid.WithSchedulerReference(reference))); err != nil {
					t.Error(err)
					return
				}
				if !scheduler.Exists(reference) {
					t.Error("注册成功的任务应可查询")
					return
				}
				if err := scheduler.Cancel(reference); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	close(start)
	wg.Wait()
	scheduler.Clear()
	for worker := range workers {
		for round := range rounds {
			reference := strconv.Itoa(worker) + "/" + strconv.Itoa(round)
			require.False(t, scheduler.Exists(reference))
		}
	}
}

func TestScheduler_ConcurrentSameReference(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Scheduler()
	const workers = 16
	for range 10 {
		var registered, cancelled atomic.Int32
		start := make(chan struct{})
		var wg sync.WaitGroup
		for range workers {
			wg.Go(func() {
				<-start
				if err := schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 1, vivid.WithSchedulerReference("same"))); err == nil {
					registered.Add(1)
				} else if !errors.Is(err, vivid.ErrorScheduledTaskAlreadyExists) {
					t.Error(err)
				}
			})
		}
		close(start)
		wg.Wait()
		require.EqualValues(t, 1, registered.Load(), "相同 reference 只允许一次注册成功")
		start = make(chan struct{})
		for range workers {
			wg.Go(func() {
				<-start
				if err := scheduler.Cancel("same"); err == nil {
					cancelled.Add(1)
				} else if !errors.Is(err, vivid.ErrorNotFound) {
					t.Error(err)
				}
			})
		}
		close(start)
		wg.Wait()
		require.EqualValues(t, 1, cancelled.Load(), "相同任务只允许一次取消成功")
		require.False(t, scheduler.Exists("same"))
	}
}

func TestScheduler_ConcurrentClear(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler.Load()
	const workers, rounds = 8, 100
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		for range rounds {
			scheduler.Clear()
		}
	})
	for worker := range workers {
		wg.Go(func() {
			<-start
			for round := range rounds {
				reference := strconv.Itoa(worker) + "/" + strconv.Itoa(round)
				if err := schedulerResultError(scheduler.Loop(system.Ref(), time.Hour, round, vivid.WithSchedulerReference(reference))); err != nil {
					t.Error(err)
					return
				}
				_ = scheduler.Exists(reference)
				if err := scheduler.Cancel(reference); err != nil && !errors.Is(err, vivid.ErrorNotFound) {
					t.Error(err)
					return
				}
			}
		})
	}
	close(start)
	wg.Wait()
	scheduler.Clear()
	for worker := range workers {
		for round := range rounds {
			reference := strconv.Itoa(worker) + "/" + strconv.Itoa(round)
			require.False(t, scheduler.Exists(reference))
		}
	}
}

type schedulerRegistrationSchedule struct {
	plan    cron.Schedule
	entered chan struct{}
	release chan struct{}
	expired bool
}

func (t *schedulerRegistrationSchedule) Next(after time.Time) time.Time {
	close(t.entered)
	<-t.release
	if t.expired {
		return time.Time{}
	}
	return t.plan.Next(after)
}

type schedulerDelayedSchedule struct{ delay time.Duration }

func (s *schedulerDelayedSchedule) Next(after time.Time) time.Time { return after.Add(s.delay) }

func TestScheduler_RegistrationAndCleanup(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "clear", true: "cancel"}[cancel], func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			scheduler := system.Context.scheduler.Load()
			trigger := &schedulerRegistrationSchedule{
				plan:    &schedulerDelayedSchedule{delay: time.Hour},
				entered: make(chan struct{}), release: make(chan struct{}),
			}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(trigger.release) }) }
			defer release()
			registered, cleaned := make(chan error, 1), make(chan error, 1)
			go func() {
				registered <- schedulerResultError(scheduler.scheduleCron(system.Ref(), 1, vivid.NewScheduleOptions(vivid.WithSchedulerReference("pending")), trigger))
			}()
			receiveRetryResult(t, trigger.entered)
			// 计时器尚未发布，取消与清理不能越过正在提交的登记。
			go func() {
				if cancel {
					cleaned <- scheduler.Cancel("pending")
				} else {
					scheduler.Clear()
					cleaned <- nil
				}
			}()
			select {
			case err := <-cleaned:
				t.Fatalf("清理提前完成，可能遗漏计时器: %v", err)
			case <-time.After(20 * time.Millisecond):
			}
			release()
			require.NoError(t, receiveRetryResult(t, registered))
			require.NoError(t, receiveRetryResult(t, cleaned))
			require.False(t, scheduler.Exists("pending"))
			// Clear 只清理任务，不关闭后续注册。
			require.NoError(t, schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("pending"))))
			require.NoError(t, scheduler.Cancel("pending"))
		})
	}
}

func TestScheduler_FailedRegistration(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler.Load()
	release := make(chan struct{})
	close(release)
	trigger := &schedulerRegistrationSchedule{entered: make(chan struct{}), release: release, expired: true}
	opts := vivid.NewScheduleOptions(vivid.WithSchedulerReference("failed"))
	task, err := scheduler.scheduleCron(system.Ref(), 1, opts, trigger)
	require.ErrorIs(t, err, vivid.ErrorScheduledTaskExpired)
	require.Nil(t, task)
	require.False(t, scheduler.Exists("failed"), "注册失败不能留下本地登记")
	require.ErrorIs(t, scheduler.Cancel("failed"), vivid.ErrorNotFound)
	require.NoError(t, schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 1, vivid.WithSchedulerReference("failed"))))
	require.ErrorIs(t, schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("failed"))), vivid.ErrorScheduledTaskAlreadyExists)
	require.True(t, scheduler.Exists("failed"), "重复注册失败不能删除已有任务的登记")
	require.NoError(t, scheduler.Cancel("failed"))
}

type schedulerReentrantLogger struct {
	log.Logger
	callback func()
}

func (l *schedulerReentrantLogger) Debug(message string, args ...any) {
	if message == "scheduler cancel scheduled" {
		l.callback()
	}
	l.Logger.Debug(message, args...)
}

func TestScheduler_CallbackReentry(t *testing.T) {
	logger := &schedulerReentrantLogger{Logger: log.NewTextLogger(log.WithLevel(log.LevelError))}
	system := NewTestSystem(t, vivid.WithActorSystemLogger(logger))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Scheduler()
	callbackResult := make(chan error, 1)
	logger.callback = func() {
		if scheduler.Exists("same") {
			callbackResult <- errors.New("取消完成后不能保留原登记")
			return
		}
		callbackResult <- schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("same")))
	}
	// 配置回调也允许查询同一 Scheduler，不应在登记锁内执行。
	require.NoError(t, schedulerResultError(scheduler.Once(system.Ref(), time.Hour, 1, func(opts *vivid.ScheduleOptions) {
		require.False(t, scheduler.Exists("same"))
		opts.Reference = "same"
	})))
	result := make(chan error, 1)
	go func() { result <- scheduler.Cancel("same") }()
	require.NoError(t, receiveRetryResult(t, result))
	require.NoError(t, receiveRetryResult(t, callbackResult), "日志回调可用相同 reference 重新注册")
	require.True(t, scheduler.Exists("same"))
	scheduler.Clear()
}

// schedulerResultError 用于仅验证注册错误契约的回归用例。
func schedulerResultError(_ vivid.ScheduledTask, err error) error {
	return err
}
