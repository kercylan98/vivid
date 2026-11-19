package actor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

type creationFixedActor struct {
	vivid.Actor
	create func()
}

func (a *creationFixedActor) FixedOptions(vivid.FixedOptionContext) []vivid.ActorOption {
	a.create()
	return nil
}

type creationLogger struct {
	log.Logger
	create  func()
	entered atomic.Bool
}

func (l *creationLogger) Debug(message string, args ...any) {
	if message == "actor spawned" && l.create != nil && l.entered.CompareAndSwap(false, true) {
		l.create()
	}
	l.Logger.Debug(message, args...)
}

func TestSystem_ActorOfReentry(t *testing.T) {
	const scenarioEnv = "VIVID_CREATION_REENTRY"
	if scenario := os.Getenv(scenarioEnv); scenario != "" {
		testActorOfReentry(t, scenario)
		return
	}
	// 回调重入可能死锁，独立进程同时约束失败时的等待和资源残留。
	for _, scenario := range []string{"prelaunch", "option", "fixed", "event", "logger", "error", "panic"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSystem_ActorOfReentry$", "-test.timeout=10s")
			command.Env = append(os.Environ(), scenarioEnv+"="+scenario)
			output, err := command.CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

func testActorOfReentry(t *testing.T, scenario string) {
	logger := &creationLogger{Logger: log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))}
	system := NewTestSystem(t, vivid.WithActorSystemLogger(logger))
	t.Cleanup(func() { _ = system.Stop(2 * time.Second) })
	var killed atomic.Int32
	behavior := vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnKilled:
			if message.Ref.Equals(ctx.Ref()) {
				killed.Add(1)
			}
		case string:
			ctx.Reply(ctx.Parent())
		}
	})
	var nested vivid.ActorRef
	create := func() {
		var err error
		nested, err = system.ActorOf(behavior, vivid.WithActorName("nested"))
		require.NoError(t, err)
	}
	var actor vivid.Actor = behavior
	options := []vivid.ActorOption{vivid.WithActorName("outer")}
	switch scenario {
	case "prelaunch", "error", "panic":
		actor = vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
			create()
			if scenario == "error" {
				return context.Canceled
			}
			if scenario == "panic" {
				panic("creation panic")
			}
			return nil
		}, behavior)
	case "option":
		options = append(options, func(*vivid.ActorOptions) { create() })
	case "fixed":
		actor = &creationFixedActor{Actor: behavior, create: create}
	case "event":
		system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
			if event, ok := event.(ves.ActorSpawnedEvent); ok && event.ActorRef.GetPath() == "/outer" {
				create()
			}
		}}
	case "logger":
		logger.create = create
	}
	if scenario == "panic" {
		require.PanicsWithValue(t, "creation panic", func() { _, _ = system.ActorOf(actor, options...) })
	} else {
		ref, err := system.ActorOf(actor, options...)
		if scenario == "error" {
			require.Nil(t, ref)
			require.ErrorIs(t, err, context.Canceled)
		} else {
			require.NoError(t, err)
		}
	}
	require.NotNil(t, nested)
	parent, err := system.Ask(nested, "parent", time.Second).Result()
	require.NoError(t, err)
	require.Same(t, system.Ref(), parent, "重入系统创建得到直属 Actor，不属于尚在初始化的 Actor")
	if scenario == "error" || scenario == "panic" {
		_, err = system.ActorOf(behavior, vivid.WithActorName("outer"))
		require.NoError(t, err, "失败创建必须释放名称与在途登记，重入成功的 Actor 独立保留")
	}
	require.Len(t, system.Children(), 2)
	require.NoError(t, system.Stop(time.Second))
	require.EqualValues(t, 2, killed.Load())
	system.actorInitLock.Lock()
	require.Empty(t, system.initializingActors)
	system.actorInitLock.Unlock()
}

func TestSystem_ActorOfParallelInitialization(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); _ = system.Stop(time.Second) })
	first := make(chan error, 1)
	go func() {
		_, err := system.ActorOf(vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error {
			close(entered)
			<-release
			return nil
		}, vivid.ActorFN(func(vivid.ActorContext) {})), vivid.WithActorName("held"))
		first <- err
	}()
	receiveRetryResult(t, entered)
	results := make(chan error, 2)
	var duplicatePrelaunch atomic.Int32
	go func() {
		_, err := system.ActorOf(vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error {
			duplicatePrelaunch.Add(1)
			return nil
		}, vivid.ActorFN(func(vivid.ActorContext) {})), vivid.WithActorName("held"))
		results <- err
	}()
	go func() {
		_, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}), vivid.WithActorName("independent"))
		results <- err
	}()
	var succeeded, duplicate int
	for range 2 {
		err := receiveRetryResult(t, results)
		if err == nil {
			succeeded++
		} else if errors.Is(err, vivid.ErrorActorAlreadyExists) {
			duplicate++
		} else {
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, succeeded, "阻塞的初始化不能阻塞无关创建")
	require.Equal(t, 1, duplicate, "名称占用期间立即拒绝同名创建")
	require.Zero(t, duplicatePrelaunch.Load())
	releaseOnce.Do(func() { close(release) })
	require.NoError(t, receiveRetryResult(t, first))
}

func TestSystem_StopSealsCreationAdmission(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); _ = system.Stop(time.Second) })
	created := make(chan error, 1)
	nested := make(chan error, 1)
	var killed atomic.Bool
	go func() {
		_, err := system.ActorOf(vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error {
			close(entered)
			<-release
			_, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
			nested <- err
			return nil
		}, vivid.ActorFN(func(ctx vivid.ActorContext) {
			if _, ok := ctx.Message().(*vivid.OnKill); ok {
				killed.Store(true)
			}
		})))
		created <- err
	}()
	receiveRetryResult(t, entered)
	require.ErrorIs(t, system.Stop(20*time.Millisecond), vivid.ErrorActorSystemStopFailed)
	late := make(chan error, 1)
	go func() {
		_, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
		late <- err
	}()
	require.ErrorIs(t, receiveRetryResult(t, late), vivid.ErrorActorSystemStopped, "关闭准入不能等待仍在执行的用户初始化")
	require.False(t, killed.Load(), "已有创建结束前不能关闭根节点")
	releaseOnce.Do(func() { close(release) })
	require.ErrorIs(t, receiveRetryResult(t, nested), vivid.ErrorActorSystemStopped, "重入创建同样遵守当前准入状态")
	require.NoError(t, receiveRetryResult(t, created))
	receiveRetryResult(t, system.stopCompleted)
	require.True(t, killed.Load())
}

func TestSystem_StopWaitsForAllCreations(t *testing.T) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))))
	rollback := &rollbackEventStream{EventStream: system.eventStream, entered: make(chan struct{}), release: make(chan struct{})}
	system.eventStream = rollback
	require.NoError(t, system.Start())
	releases := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	var releaseOnce [3]sync.Once
	var rollbackOnce sync.Once
	t.Cleanup(func() {
		for index := range releases {
			releaseOnce[index].Do(func() { close(releases[index]) })
		}
		rollbackOnce.Do(func() { close(rollback.release) })
		_ = system.Stop(time.Second)
	})
	type result struct {
		index int
		err   error
		panic any
	}
	entered := make(chan int, 3)
	results := make(chan result, 3)
	var killed atomic.Int32
	for index, name := range []string{"success", "rollback", "panic"} {
		go func() {
			outcome := result{index: index}
			defer func() { outcome.panic = recover(); results <- outcome }()
			_, outcome.err = system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
				entered <- index
				<-releases[index]
				switch index {
				case 1:
					return context.Canceled
				case 2:
					panic("creation panic")
				}
				return nil
			}, vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(*vivid.OnKill); ok {
					killed.Add(1)
				}
			})), vivid.WithActorName(name))
		}()
	}
	for range releases {
		receiveRetryResult(t, entered)
	}
	require.ErrorIs(t, system.Stop(20*time.Millisecond), vivid.ErrorActorSystemStopFailed)
	releaseOnce[0].Do(func() { close(releases[0]) })
	first := receiveRetryResult(t, results)
	require.Equal(t, 0, first.index)
	require.NoError(t, first.err)
	require.Nil(t, first.panic)
	require.Zero(t, killed.Load(), "一个创建完成不能提前开始根节点关闭")
	releaseOnce[1].Do(func() { close(releases[1]) })
	receiveRetryResult(t, rollback.entered)
	require.ErrorIs(t, system.Stop(10*time.Millisecond), vivid.ErrorActorSystemStopFailed, "失败创建的回滚也属于在途工作")
	require.NoError(t, system.options.Context.Err())
	rollbackOnce.Do(func() { close(rollback.release) })
	second := receiveRetryResult(t, results)
	require.Equal(t, 1, second.index)
	require.ErrorIs(t, second.err, context.Canceled)
	require.Nil(t, second.panic)
	require.Zero(t, killed.Load(), "panic 创建结束前继续等待")
	releaseOnce[2].Do(func() { close(releases[2]) })
	third := receiveRetryResult(t, results)
	require.Equal(t, 2, third.index)
	require.Equal(t, "creation panic", third.panic)
	receiveRetryResult(t, system.stopCompleted)
	require.EqualValues(t, 1, killed.Load(), "只有初始化成功的 Actor 执行终止生命周期")
	system.actorInitLock.Lock()
	require.Empty(t, system.initializingActors)
	system.actorInitLock.Unlock()
}

func BenchmarkSystem_ActorOfParallel(b *testing.B) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))))
	if err := system.Start(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := system.Stop(5 * time.Second); err != nil {
			b.Error(err)
		}
	})
	behavior := vivid.ActorFN(func(vivid.ActorContext) {})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := system.ActorOf(behavior); err != nil {
				b.Error(err)
			}
		}
	})
}
