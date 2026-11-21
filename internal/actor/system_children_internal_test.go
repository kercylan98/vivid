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

func TestSystem_ConcurrentRootChildren(t *testing.T) {
	const scenarioEnv = "VIVID_ROOT_CHILDREN_SCENARIO"
	if scenario := os.Getenv(scenarioEnv); scenario != "" {
		testConcurrentRootChildren(t, scenario == "stop")
		return
	}
	// map 并发写入会直接退出进程，每种交错在独立进程中验证。
	for _, scenario := range []string{"churn", "stop"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSystem_ConcurrentRootChildren$", "-test.timeout=25s")
			command.Env = append(os.Environ(), scenarioEnv+"="+scenario)
			output, err := command.CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

func testConcurrentRootChildren(t *testing.T, stopDuringCreation bool) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))))
	t.Cleanup(func() { _ = system.Stop(5 * time.Second) })
	var created, launched, killed, finished atomic.Int32
	firstCreated := make(chan struct{})
	var firstOnce sync.Once
	behavior := vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			launched.Add(1)
			ctx.Kill(ctx.Ref(), false, "short lived actor")
		case *vivid.OnKill:
			killed.Add(1)
		case *vivid.OnKilled:
			if message.Ref.Equals(ctx.Ref()) {
				finished.Add(1)
			}
		}
	})
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 2000 {
				_, err := system.ActorOf(behavior)
				if stopDuringCreation && errors.Is(err, vivid.ErrorActorSystemStopped) {
					return
				}
				if err != nil {
					t.Error(err)
					return
				}
				created.Add(1)
				firstOnce.Do(func() { close(firstCreated) })
			}
		})
	}
	// 快照与创建、终止同时进行，结果只读本次取得的独立切片。
	reading := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-reading:
				return
			default:
			}
			for _, child := range system.Children() {
				if child == nil || child.GetPath() == "/" {
					t.Error("根级快照只能包含有效子 Actor")
				}
			}
		}
	}()
	if stopDuringCreation {
		receiveRetryResult(t, firstCreated)
		require.NoError(t, system.Stop(5*time.Second))
	}
	workers.Wait()
	close(reading)
	receiveRetryResult(t, readerDone)
	if !stopDuringCreation {
		require.EqualValues(t, 16000, created.Load())
		require.NoError(t, system.Stop(5*time.Second))
	}
	require.Positive(t, created.Load())
	require.Equal(t, created.Load(), launched.Load())
	require.Equal(t, created.Load(), killed.Load())
	require.Equal(t, created.Load(), finished.Load())
	require.Empty(t, system.Children())
	remaining := 0
	system.actorContexts.Range(func(_, _ any) bool { remaining++; return true })
	require.Zero(t, remaining, "停止完成后不能遗漏子 Actor")
}

func TestSystem_RootChildrenCallbackSnapshot(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	deathSnapshots := make(chan vivid.ActorRefs, 1)
	behavior := system.behaviorStack.Peek()
	system.behaviorStack.Push(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(*vivid.OnKilled); ok && !message.Ref.Equals(ctx.Ref()) {
			deathSnapshots <- ctx.Children()
		}
		behavior(ctx)
	})
	snapshots := make(chan vivid.ActorRefs, 1)
	system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
		if _, ok := event.(ves.ActorSpawnedEvent); ok {
			snapshots <- system.Children()
		}
	}}
	created := make(chan vivid.ActorRef, 1)
	go func() {
		ref, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
		if err != nil {
			t.Error(err)
		}
		created <- ref
	}()
	ref := receiveRetryResult(t, created)
	snapshot := receiveRetryResult(t, snapshots)
	require.Equal(t, vivid.ActorRefs{ref}, snapshot, "事件回调中查询不能等待创建锁")
	snapshot[0] = nil
	require.Equal(t, vivid.ActorRefs{ref}, system.Children(), "修改返回切片不能影响内部名单")
	system.Kill(ref, false)
	require.Empty(t, receiveRetryResult(t, deathSnapshots), "终止行为中查询不能持有子节点表锁")
}

func TestSystem_RootChildrenStaleDeath(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	var replacement vivid.ActorRef
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		if replacement != nil {
			system.Kill(replacement, false)
		}
		_ = system.Stop(time.Second)
	})
	// 固定根邮箱尚未消费旧通知的窗口，新实例已在系统索引中取得相同名称。
	behavior := system.behaviorStack.Peek()
	system.behaviorStack.Push(func(ctx vivid.ActorContext) {
		if callback, ok := ctx.Message().(func(vivid.ActorContext)); ok {
			callback(ctx)
			return
		}
		behavior(ctx)
	})
	system.tell(true, system.Ref(), func(vivid.ActorContext) { close(entered); <-release })
	receiveRetryResult(t, entered)
	oldKilled := make(chan struct{}, 1)
	system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
		if event, ok := event.(ves.ActorKilledEvent); ok && event.ActorRef.GetPath() == "/reused" {
			oldKilled <- struct{}{}
		}
	}}
	old, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}), vivid.WithActorName("reused"))
	require.NoError(t, err)
	system.Kill(old, false)
	receiveRetryResult(t, oldKilled)
	var finished atomic.Int32
	replacement, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(*vivid.OnKilled); ok && message.Ref.Equals(ctx.Ref()) {
			finished.Add(1)
		}
	}), vivid.WithActorName("reused"))
	require.NoError(t, err)
	require.NotSame(t, old, replacement)
	checked := make(chan vivid.ActorRefs, 1)
	system.tell(true, system.Ref(), func(ctx vivid.ActorContext) { checked <- ctx.Children() })
	releaseOnce.Do(func() { close(release) })
	snapshot := receiveRetryResult(t, checked)
	require.Len(t, snapshot, 1, "旧通知不能删除同路径的新实例")
	require.Same(t, replacement, snapshot[0])
	require.NoError(t, system.Stop(time.Second))
	require.EqualValues(t, 1, finished.Load(), "同路径重建的实例必须纳入关闭")
	require.Same(t, replacement, snapshot[0], "后续终止不能修改已有快照")
}

func BenchmarkSystem_ActorOf(b *testing.B) {
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
	for range b.N {
		if _, err := system.ActorOf(behavior); err != nil {
			b.Fatal(err)
		}
	}
}
