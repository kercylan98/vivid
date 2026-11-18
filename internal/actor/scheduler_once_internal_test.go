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

func TestScheduler_OnceReleasesRegistration(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "generated", true: "explicit"}[explicit], func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			const count = 128
			delivered := make(chan int, count)
			target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if message, ok := ctx.Message().(int); ok {
					delivered <- message
				}
			}))
			require.NoError(t, err)
			for i := range count {
				var options []vivid.ScheduleOption
				if explicit {
					options = append(options, vivid.WithSchedulerReference(strconv.Itoa(i)))
				}
				require.NoError(t, system.Scheduler().Once(target, time.Millisecond, i, options...))
			}
			seen := make(map[int]bool, count)
			for range count {
				message := receiveRetryResult(t, delivered)
				require.False(t, seen[message], "一次任务不能重复投递")
				seen[message] = true
			}
			scheduler := system.Context.scheduler.Load()
			scheduler.mu.Lock()
			retained := len(scheduler.tasks)
			scheduler.mu.Unlock()
			require.Zero(t, retained, "已投递的一次任务不能继续保留登记")
			if explicit {
				require.False(t, scheduler.Exists("0"))
				require.ErrorIs(t, scheduler.Cancel("0"), vivid.ErrorNotFound)
			}
		})
	}
}

func TestScheduler_OnceReusesReferenceInReceiver(t *testing.T) {
	for _, delay := range []time.Duration{0, -time.Second, time.Millisecond} {
		t.Run(delay.String(), func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			const count = 32
			result := make(chan error, 1)
			_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				var next int
				switch message := ctx.Message().(type) {
				case *vivid.OnLaunch:
				case int:
					if ctx.Scheduler().Exists("next") {
						result <- errors.New("已取出投递的任务仍有登记")
						return
					}
					next = message + 1
					if next == count {
						result <- nil
						return
					}
				default:
					return
				}
				if err := ctx.Scheduler().Once(ctx.Ref(), delay, next, vivid.WithSchedulerReference("next")); err != nil {
					result <- err
				}
			}))
			require.NoError(t, err)
			require.NoError(t, receiveRetryResult(t, result))
		})
	}
}

type onceObservedRef struct {
	vivid.ActorRef
	called atomic.Int32
	panic  bool
}

func (r *onceObservedRef) Equals(other vivid.ActorRef) bool {
	r.called.Add(1)
	if r.panic {
		panic("once delivery")
	}
	return r.ActorRef.Equals(other)
}

func TestScheduler_OnceLateCallback(t *testing.T) {
	for _, action := range []string{"cancel", "clear", "close"} {
		t.Run(action, func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
			scheduler := system.Context.scheduler.Load()
			receiver := &onceObservedRef{ActorRef: system.Ref()}
			opts := vivid.NewScheduleOptions(vivid.WithSchedulerReference("same"))
			require.NoError(t, scheduler.Once(receiver, time.Hour, 1, vivid.WithSchedulerReference("same")))
			scheduler.mu.Lock()
			old := scheduler.tasks["same"]
			scheduler.mu.Unlock()
			switch action {
			case "cancel":
				require.NoError(t, scheduler.Cancel("same"))
			case "clear":
				scheduler.Clear()
			case "close":
				scheduler.close()
			}
			require.False(t, old.once.timer.Stop(), "取消、清理与关闭都应停止原计时器")
			current := scheduler
			if action == "close" {
				current = newScheduler(system.Context)
				defer current.close()
			}
			require.NoError(t, current.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("same")))
			// 固定重放已启动但尚未取得登记的旧回调，不能投递或删除同名新任务。
			scheduler.fireOnce(old.once, receiver, 1, opts)
			require.Zero(t, receiver.called.Load())
			require.True(t, current.Exists("same"))
			require.NoError(t, current.Cancel("same"))
		})
	}
}

func TestScheduler_OnceSharesReferenceScope(t *testing.T) {
	for _, first := range []string{"once", "loop", "cron"} {
		for _, second := range []string{"once", "loop", "cron"} {
			t.Run(first+"/"+second, func(t *testing.T) {
				system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
				t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
				scheduler := system.Context.scheduler.Load()
				register := func(kind string) error {
					option := vivid.WithSchedulerReference("shared")
					switch kind {
					case "once":
						return scheduler.Once(system.Ref(), time.Hour, 1, option)
					case "loop":
						return scheduler.Loop(system.Ref(), time.Hour, 1, option)
					default:
						return scheduler.Cron(system.Ref(), "0 0 0 1 1 ? 2099", 1, option)
					}
				}
				require.NoError(t, register(first))
				require.ErrorIs(t, register(second), quartz.ErrJobAlreadyExists)
				require.NoError(t, scheduler.Cancel("shared"))
				require.NoError(t, register(second), "取消后可以跨任务类型复用 reference")
				require.NoError(t, scheduler.Cancel("shared"))
			})
		}
	}
}

type onceFailureLogger struct {
	log.Logger
	failed chan struct{}
}

func (l *onceFailureLogger) Error(message string, args ...any) {
	if message == "scheduler once delivery failed" {
		l.failed <- struct{}{}
	}
}

func TestScheduler_OnceDeliveryPanic(t *testing.T) {
	logger := &onceFailureLogger{Logger: log.NewTextLogger(log.WithLevel(log.LevelError)), failed: make(chan struct{}, 1)}
	system := NewTestSystem(t, vivid.WithActorSystemLogger(logger))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	scheduler := system.Scheduler()
	require.NoError(t, scheduler.Once(&onceObservedRef{ActorRef: system.Ref(), panic: true}, 0, 1, vivid.WithSchedulerReference("failed")))
	receiveRetryResult(t, logger.failed)
	require.False(t, scheduler.Exists("failed"), "投递 panic 也不能保留登记")
	require.NoError(t, scheduler.Once(system.Ref(), time.Hour, 2, vivid.WithSchedulerReference("failed")))
	require.NoError(t, scheduler.Cancel("failed"))
	require.ErrorIs(t, scheduler.Once(nil, time.Hour, 1), vivid.ErrorIllegalArgument)
	require.ErrorIs(t, scheduler.Once(system.Ref(), time.Hour, 1, vivid.WithScheduleOptions(vivid.ScheduleOptions{})), vivid.ErrorIllegalArgument)
}

type onceClaimedRef struct {
	vivid.ActorRef
	entered chan struct{}
	release chan struct{}
}

func (r *onceClaimedRef) Equals(other vivid.ActorRef) bool {
	close(r.entered)
	<-r.release
	return r.ActorRef.Equals(other)
}

func TestScheduler_OnceClaimPrecedesDelivery(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
	ready := make(chan vivid.ActorContext, 1)
	delivered := make(chan int, 1)
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			ready <- ctx
		case int:
			delivered <- message
		}
	}))
	require.NoError(t, err)
	ctx := receiveRetryResult(t, ready)
	receiver := &onceClaimedRef{ActorRef: ctx.Ref(), entered: make(chan struct{}), release: make(chan struct{})}
	var once sync.Once
	resume := func() { once.Do(func() { close(receiver.release) }) }
	defer resume()
	scheduler := ctx.Scheduler()
	require.NoError(t, scheduler.Once(receiver, 0, 1, vivid.WithSchedulerReference("same")))
	receiveRetryResult(t, receiver.entered)
	require.False(t, scheduler.Exists("same"), "投递尚未完成时登记已释放")
	require.ErrorIs(t, scheduler.Cancel("same"), vivid.ErrorNotFound)
	require.NoError(t, scheduler.Once(ctx.Ref(), time.Hour, 2, vivid.WithSchedulerReference("same")))
	resume()
	require.Equal(t, 1, receiveRetryResult(t, delivered), "先取出的一次任务仍可完成投递")
	require.True(t, scheduler.Exists("same"), "旧任务投递完成不能删除新登记")
	require.NoError(t, scheduler.Cancel("same"))
}

func TestScheduler_RemoteDelivery(t *testing.T) {
	for _, kind := range []string{"once", "loop", "cron"} {
		t.Run(kind, func(t *testing.T) { testSchedulerRemoteDelivery(t, kind) })
	}
}

func testSchedulerRemoteDelivery(t *testing.T, kind string) {
	newSystem := func() *System {
		system := NewTestSystem(t,
			vivid.WithActorSystemRemoting(admissionSystemAddress(t)),
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		)
		t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
		return system.System
	}
	client, server := newSystem(), newSystem()
	target, err := server.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(vivid.ActorRef); ok && message.GetPath() == "/tick" {
			ctx.Reply(message)
		}
	}))
	require.NoError(t, err)
	tick, err := client.CreateRef(client.Ref().GetAddress(), "/tick")
	require.NoError(t, err)
	result := make(chan error, 1)
	var replied bool
	_, err = client.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			var err error
			option := vivid.WithSchedulerReference("remote")
			switch kind {
			case "once":
				err = ctx.Scheduler().Once(target, 0, tick, option)
			case "loop":
				err = ctx.Scheduler().Loop(target, time.Millisecond, tick, option)
			case "cron":
				err = ctx.Scheduler().Cron(target, "* * * * * *", tick, option)
			}
			if err != nil {
				result <- err
			}
		case vivid.ActorRef:
			if replied {
				return
			}
			replied = true
			if !message.Equals(tick) {
				result <- errors.New("远程应答或到期登记不正确")
				return
			}
			if kind == "once" {
				if ctx.Scheduler().Exists("remote") {
					result <- errors.New("一次任务仍保留登记")
					return
				}
			} else if err := ctx.Scheduler().Cancel("remote"); err != nil {
				result <- err
				return
			}
			result <- nil
		}
	}))
	require.NoError(t, err)
	require.NoError(t, receiveRetryResult(t, result), "调度保留发送方与真实 TCP 应答语义")
}
