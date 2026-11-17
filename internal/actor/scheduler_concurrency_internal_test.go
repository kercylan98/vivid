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
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/require"
)

func TestScheduler_ConcurrentRegistration(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler
	const workers, rounds = 16, 100
	start := make(chan struct{})
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			<-start
			for round := range rounds {
				reference := strconv.Itoa(worker) + "/" + strconv.Itoa(round)
				if err := scheduler.Once(system.Ref(), time.Hour, round, vivid.WithSchedulerReference(reference)); err != nil {
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
			require.Error(t, system.scheduler.DeleteJob(scheduler.jobKey(reference)), "取消后不能残留底层任务")
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
				if err := scheduler.Once(system.Ref(), time.Hour, 1, vivid.WithSchedulerReference("same")); err == nil {
					registered.Add(1)
				} else if !errors.Is(err, quartz.ErrJobAlreadyExists) {
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
	scheduler := system.Context.scheduler
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
				if err := scheduler.Once(system.Ref(), time.Hour, round, vivid.WithSchedulerReference(reference)); err != nil {
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
			require.Error(t, system.scheduler.DeleteJob(scheduler.jobKey(reference)), "并发清理后不能残留底层任务")
		}
	}
}

type schedulerRegistrationTrigger struct {
	quartz.Trigger
	entered chan struct{}
	release chan struct{}
	err     error
}

func (t *schedulerRegistrationTrigger) NextFireTime(prev int64) (int64, error) {
	close(t.entered)
	<-t.release
	if t.err != nil {
		return 0, t.err
	}
	return t.Trigger.NextFireTime(prev)
}

func TestScheduler_RegistrationAndCleanup(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "clear", true: "cancel"}[cancel], func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			scheduler := system.Context.scheduler
			trigger := &schedulerRegistrationTrigger{
				Trigger: quartz.NewRunOnceTrigger(time.Hour),
				entered: make(chan struct{}), release: make(chan struct{}),
			}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(trigger.release) }) }
			defer release()
			registered, cleaned := make(chan error, 1), make(chan error, 1)
			go func() {
				registered <- scheduler.scheduleJob(system.Ref(), 1, vivid.NewScheduleOptions(vivid.WithSchedulerReference("pending")), trigger, "once")
			}()
			receiveRetryResult(t, trigger.entered)
			// 底层尚未注册，取消与清理不能越过正在提交的登记。
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
				t.Fatalf("清理提前完成，可能遗漏底层任务: %v", err)
			case <-time.After(20 * time.Millisecond):
			}
			release()
			require.NoError(t, receiveRetryResult(t, registered))
			require.NoError(t, receiveRetryResult(t, cleaned))
			require.False(t, scheduler.Exists("pending"))
			require.Error(t, system.scheduler.DeleteJob(scheduler.jobKey("pending")))
			// Clear 只清理任务，不关闭后续注册。
			require.NoError(t, scheduler.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("pending")))
			require.NoError(t, scheduler.Cancel("pending"))
		})
	}
}

func TestScheduler_FailedRegistration(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Context.scheduler
	rejected := errors.New("schedule rejected")
	release := make(chan struct{})
	close(release)
	trigger := &schedulerRegistrationTrigger{entered: make(chan struct{}), release: release, err: rejected}
	opts := vivid.NewScheduleOptions(vivid.WithSchedulerReference("failed"))
	require.ErrorIs(t, scheduler.scheduleJob(system.Ref(), 1, opts, trigger, "once"), rejected)
	require.False(t, scheduler.Exists("failed"), "底层注册失败不能留下本地登记")
	require.ErrorIs(t, scheduler.Cancel("failed"), vivid.ErrorNotFound)
	require.NoError(t, scheduler.Once(system.Ref(), time.Hour, 1, vivid.WithSchedulerReference("failed")))
	require.ErrorIs(t, scheduler.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("failed")), quartz.ErrJobAlreadyExists)
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
		callbackResult <- scheduler.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("same"))
	}
	// 配置回调也允许查询同一 Scheduler，不应在登记锁内执行。
	require.NoError(t, scheduler.Once(system.Ref(), time.Hour, 1, func(opts *vivid.ScheduleOptions) {
		require.False(t, scheduler.Exists("same"))
		opts.Reference = "same"
	}))
	result := make(chan error, 1)
	go func() { result <- scheduler.Cancel("same") }()
	require.NoError(t, receiveRetryResult(t, result))
	require.NoError(t, receiveRetryResult(t, callbackResult), "日志回调可用相同 reference 重新注册")
	require.True(t, scheduler.Exists("same"))
	scheduler.Clear()
}
