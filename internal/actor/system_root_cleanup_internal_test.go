package actor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func TestSystem_StopWaitsForRootCleanup(t *testing.T) {
	for _, checkpoint := range []string{"log", "event"} {
		t.Run(checkpoint, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			resume := func() { releaseOnce.Do(func() { close(release) }) }
			block := func() {
				close(entered)
				<-release
			}
			var killed atomic.Int32
			logger := &loggerProbe{write: func(_, message string, args []any) {
				if checkpoint != "log" || message != "actor killed" {
					return
				}
				for _, arg := range args {
					if attr, ok := arg.(log.Attr); ok && attr.Key == "path" && attr.Value.String() == "/" {
						block()
						return
					}
				}
			}}
			system := NewSystem(vivid.WithActorSystemLogger(logger))
			stream := system.eventStream.(*eventStream)
			system.eventStream = &startupEventStream{EventStream: stream, observe: func(event vivid.Message) {
				if event, ok := event.(ves.ActorKilledEvent); ok && event.ActorRef.GetPath() == "/" {
					killed.Add(1)
					if checkpoint == "event" {
						block()
					}
				}
			}}
			t.Cleanup(func() {
				resume()
				_ = system.Stop(time.Second)
				receiveRetryResult(t, system.stopCompleted)
			})
			require.NoError(t, system.Start())
			stream.Subscribe(system, 0)
			task, err := system.Scheduler().Once(system.Ref(), time.Hour, "pending", vivid.WithSchedulerReference("pending"))
			require.NoError(t, err)
			pending := system.Ask(system.Ref(), "unanswered", 0)

			first := make(chan error, 1)
			go func() { first <- system.Stop(100 * time.Millisecond) }()
			receiveRetryResult(t, entered)
			err = receiveRetryResult(t, first)
			require.ErrorIs(t, err, vivid.ErrorActorSystemStopFailed, "根清理尚未返回时不能报告停止成功")
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.NoError(t, system.options.Context.Err())
			require.False(t, system.requestsStopped())
			require.False(t, task.Exists())
			require.ErrorIs(t, pending.Wait(), vivid.ErrorActorDeaded)
			if checkpoint == "log" {
				require.False(t, system.ref.(*Ref).events.closed.Load())
				// 收尾尚未关闭订阅准入，新登记也必须纳入后续清理。
				stream.Subscribe(system, "late subscription")
			} else {
				requireEventStreamRejected(t, stream, system)
				_, found := system.actorContexts.Load(system.ref.GetPath())
				require.False(t, found)
			}
			require.ErrorIs(t, system.Stop(10*time.Millisecond), vivid.ErrorActorSystemStopFailed, "再次等待不能绕过仍在执行的根清理")
			second := make(chan error, 1)
			go func() { second <- system.Stop(0) }()
			resume()
			err = receiveRetryResult(t, second)
			// 后启动的调用可能在关闭完成后才获得执行机会。
			if err != nil {
				require.ErrorIs(t, err, vivid.ErrorActorSystemAlreadyStopped)
			}
			receiveRetryResult(t, system.stopCompleted)
			require.EqualValues(t, 1, killed.Load())
			require.ErrorIs(t, system.options.Context.Err(), context.Canceled)
			requireEventStreamRejected(t, stream, system)
			requireEventStreamSubscriptions(t, stream, system.ref)
			_, found := system.actorContexts.Load(system.ref.GetPath())
			require.False(t, found)
			system.futureLock.RLock()
			require.Empty(t, system.futureAgents)
			system.futureLock.RUnlock()
		})
	}
}

func TestSystem_StopWithoutRoot(t *testing.T) {
	system := NewSystem(
		vivid.WithActorSystemLogger(&loggerProbe{}),
		vivid.WithActorSystemRemoting("invalid"),
		vivid.WithActorSystemStopTimeout(time.Second),
	)
	err := system.Start()
	require.ErrorIs(t, err, vivid.ErrorActorSystemStartFailed)
	require.NotErrorIs(t, err, vivid.ErrorActorSystemStopFailed)
	require.Nil(t, system.Context)
	receiveRetryResult(t, system.stopCompleted)
	require.ErrorIs(t, system.Stop(time.Second), vivid.ErrorActorSystemAlreadyStopped)
}

func TestSystem_RootRestartDoesNotCompleteStop(t *testing.T) {
	system := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{}))
	restarted := make(chan struct{})
	system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
		if event, ok := event.(ves.ActorRestartedEvent); ok && event.ActorRef.GetPath() == "/" {
			close(restarted)
		}
	}}
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	require.NoError(t, system.Start())
	system.AdvancedTell(true, system.Ref(), &RestartMessage{Reason: "root cleanup boundary"})
	receiveRetryResult(t, restarted)
	select {
	case <-system.rootCleanupCompleted:
		t.Fatal("重启释放旧资源不代表根 Actor 最终终止")
	default:
	}
	require.NoError(t, system.Stop(time.Second))
	receiveRetryResult(t, system.rootCleanupCompleted)
}
