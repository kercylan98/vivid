package actor

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestSystem_StopTreeBeforePriorityComponents(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	phaseCompleted := make(chan struct{})
	var phaseOnce sync.Once
	componentReleases := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	var componentOnce [3]sync.Once
	t.Cleanup(func() {
		phaseOnce.Do(func() { close(phaseCompleted) })
		for index := range componentReleases {
			componentOnce[index].Do(func() { close(componentReleases[index]) })
		}
		_ = system.Stop(5 * time.Second)
		receiveRetryResult(t, system.stopCompleted)
	})

	const count = 1024
	var launched sync.WaitGroup
	launched.Add(count)
	var kills, finished atomic.Int32
	business := vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			launched.Done()
		case *vivid.OnKill:
			kills.Add(1)
		case *vivid.OnKilled:
			if message.Ref.Equals(ctx.Ref()) {
				finished.Add(1)
			}
		}
	})
	for range count {
		_, err := system.ActorOf(business)
		require.NoError(t, err)
	}
	launched.Wait()

	ready := make(chan *Context, 2)
	phaseStarted := make(chan struct{})
	childStopped := make(chan struct{})
	childBehavior := vivid.Behavior(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			ready <- ctx.(*Context)
		case *vivid.OnKill:
			kills.Add(1)
			close(phaseStarted)
		case *vivid.OnKilled:
			if message.Ref.Equals(ctx.Ref()) {
				finished.Add(1)
				close(childStopped)
			}
		}
	})
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			_, err := ctx.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
				return ctx.WithPhaseKill(phaseCompleted, 5*time.Second, childBehavior)
			}, vivid.ActorFN(childBehavior)))
			if err != nil {
				t.Error(err)
			}
			ready <- ctx.(*Context)
		case *vivid.OnKill:
			kills.Add(1)
		case *vivid.OnKilled:
			if message.Ref.Equals(ctx.Ref()) {
				select {
				case <-childStopped:
				default:
					t.Error("父 Actor 必须等待子 Actor 收尾")
				}
				finished.Add(1)
			}
		}
	}))
	require.NoError(t, err)
	for range 2 {
		ctx := receiveRetryResult(t, ready)
		ctx.mailbox.Pause()
		t.Cleanup(ctx.mailbox.Resume)
	}

	componentStarted := make(chan int, 3)
	var componentsFinished atomic.Int32
	for _, index := range []int{1, 2, 0} {
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch message := ctx.Message().(type) {
			case *vivid.OnKill:
				if finished.Load() != count+2 || componentsFinished.Load() != int32(index) {
					t.Error("业务子树收尾后，组件才能按优先级串行停止")
				}
				componentStarted <- index
				<-componentReleases[index]
			case *vivid.OnKilled:
				if message.Ref.Equals(ctx.Ref()) {
					componentsFinished.Add(1)
				}
			}
		}))
		require.NoError(t, err)
		system.tell(true, system.Ref(), &guard.RegisterStopPriority{ActorRef: ref, Priority: 2 - index})
	}
	first := make(chan error, 1)
	go func() { first <- system.Stop(20 * time.Millisecond) }()
	receiveRetryResult(t, phaseStarted)
	require.ErrorIs(t, receiveRetryResult(t, first), vivid.ErrorActorSystemStopFailed)
	select {
	case index := <-componentStarted:
		t.Fatalf("业务子树仍在收尾，不能提前停止组件 %d", index)
	default:
	}
	phaseOnce.Do(func() { close(phaseCompleted) })
	for index := range componentReleases {
		require.Equal(t, index, receiveRetryResult(t, componentStarted))
		componentOnce[index].Do(func() { close(componentReleases[index]) })
	}
	receiveRetryResult(t, system.stopCompleted)
	require.EqualValues(t, count+2, kills.Load())
	require.EqualValues(t, count+2, finished.Load())
	require.EqualValues(t, 3, componentsFinished.Load())
	remaining := 0
	system.actorContexts.Range(func(_, _ any) bool { remaining++; return true })
	require.Zero(t, remaining, "停止完成前必须释放全部子 Actor 上下文")
}

// BenchmarkSystem_StopActors 只计量完整停止，创建及启动等待不计入结果。
func BenchmarkSystem_StopActors(b *testing.B) {
	for _, count := range []int{1000, 10000, 100000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			b.StopTimer()
			for range b.N {
				system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))))
				if err := system.Start(); err != nil {
					b.Fatal(err)
				}
				var launched sync.WaitGroup
				launched.Add(count)
				var kills, finished atomic.Int32
				behavior := vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch message := ctx.Message().(type) {
					case *vivid.OnLaunch:
						launched.Done()
					case *vivid.OnKill:
						kills.Add(1)
					case *vivid.OnKilled:
						if message.Ref.Equals(ctx.Ref()) {
							finished.Add(1)
						}
					}
				})
				for range count {
					if _, err := system.ActorOf(behavior); err != nil {
						_ = system.Stop(30 * time.Second)
						b.Fatal(err)
					}
				}
				launched.Wait()
				b.StartTimer()
				err := system.Stop(30 * time.Second)
				b.StopTimer()
				if err != nil {
					b.Fatal(err)
				}
				if kills.Load() != int32(count) || finished.Load() != int32(count) {
					b.Fatal("停止成功前必须完成全部 Actor 收尾")
				}
			}
		})
	}
}
