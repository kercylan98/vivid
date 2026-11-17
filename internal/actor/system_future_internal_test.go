package actor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func newRegistrationFuture(t *testing.T, system *System, timeout time.Duration) (*AgentRef, *future.Future[vivid.Message], <-chan struct{}) {
	t.Helper()
	agent, err := NewAgentRef(&Ref{address: LocalAddress, path: "/owner"})
	require.NoError(t, err)
	cleaned := make(chan struct{})
	f := future.NewFuture[vivid.Message](nil, timeout, func() {
		system.removeFuture(agent)
		close(cleaned)
	})
	return agent, f, cleaned
}

func requireFutureUnregistered(t *testing.T, system *System, agent *AgentRef) {
	t.Helper()
	_, exists := system.actorContexts.Load(agent.ref.GetPath())
	require.False(t, exists, "已结束请求不能保留路由索引")
	system.futureLock.RLock()
	defer system.futureLock.RUnlock()
	require.Empty(t, system.futureAgents[agent.agent.GetPath()], "已结束请求不能保留所属 Actor 的索引")
}

func TestSystem_FutureRegistrationCompleted(t *testing.T) {
	for _, completion := range []string{"reply", "close", "timeout"} {
		t.Run(completion, func(t *testing.T) {
			system := &System{futureAgents: make(map[vivid.ActorPath]map[vivid.ActorPath]*AgentRef)}
			var timeout time.Duration
			if completion == "timeout" {
				timeout = time.Nanosecond
			}
			agent, f, cleaned := newRegistrationFuture(t, system, timeout)
			switch completion {
			case "reply":
				f.EnqueueMessage("ready")
			case "close":
				f.Close(context.Canceled)
			}
			receiveRetryResult(t, cleaned)
			// 固定完成回调已先于注册结束的窗口，不依赖极短超时能否抢占构造函数。
			system.appendFuture(agent, f, nil, 0)
			requireFutureUnregistered(t, system, agent)
			result, err := f.Result()
			switch completion {
			case "reply":
				require.NoError(t, err)
				require.Equal(t, "ready", result)
			case "close":
				require.ErrorIs(t, err, context.Canceled)
			case "timeout":
				require.ErrorIs(t, err, vivid.ErrorFutureTimeout)
			}
		})
	}
}

func TestSystem_FutureRegistrationConcurrent(t *testing.T) {
	for _, completion := range []string{"reply", "close", "timeout", "actor closing"} {
		t.Run(completion, func(t *testing.T) {
			for range 200 {
				system := &System{futureAgents: make(map[vivid.ActorPath]map[vivid.ActorPath]*AgentRef)}
				var timeout time.Duration
				if completion == "timeout" {
					timeout = time.Nanosecond
				}
				agent, f, cleaned := newRegistrationFuture(t, system, timeout)
				if completion == "actor closing" {
					system.appendFuture(agent, f, nil, 0)
				}
				start := make(chan struct{})
				var workers sync.WaitGroup
				workers.Add(2)
				go func() {
					defer workers.Done()
					<-start
					if completion == "actor closing" {
						system.removeFuturesByAgentPath(agent.agent.GetPath(), vivid.ErrorActorDeaded)
					} else {
						system.appendFuture(agent, f, nil, 0)
					}
				}()
				go func() {
					defer workers.Done()
					<-start
					switch completion {
					case "reply", "actor closing":
						f.EnqueueMessage("ready")
					case "close":
						f.Close(context.Canceled)
					}
				}()
				close(start)
				workers.Wait()
				receiveRetryResult(t, cleaned)
				requireFutureUnregistered(t, system, agent)
			}
		})
	}
}

func TestContext_FuturesRemovedOnSystemStop(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() {
		if err := system.Stop(time.Second); err != nil {
			require.ErrorIs(t, err, vivid.ErrorActorSystemAlreadyStopped)
		}
	})
	target, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
	require.NoError(t, err)
	requests := make(chan vivid.Future[vivid.Message], 2)
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	owner, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); !ok {
			return
		}
		requests <- ctx.Ask(target, "pending", 0)
		requests <- ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
			close(started)
			<-release
			defer close(finished)
			return "late result", nil
		}))
	}))
	require.NoError(t, err)
	ask, task := receiveRetryResult(t, requests), receiveRetryResult(t, requests)
	receiveRetryResult(t, started)
	system.futureLock.RLock()
	paths := make([]vivid.ActorPath, 0, len(system.futureAgents[owner.GetPath()]))
	for path := range system.futureAgents[owner.GetPath()] {
		paths = append(paths, path)
	}
	system.futureLock.RUnlock()
	require.Len(t, paths, 2)
	require.NoError(t, system.Stop(time.Second), "批量关闭不能持索引锁调用 Future 回调")
	require.ErrorIs(t, ask.Wait(), vivid.ErrorActorDeaded)
	require.ErrorIs(t, task.Wait(), vivid.ErrorActorDeaded)
	for _, path := range paths {
		_, exists := system.actorContexts.Load(path)
		require.False(t, exists)
	}
	system.futureLock.RLock()
	require.Empty(t, system.futureAgents[owner.GetPath()])
	system.futureLock.RUnlock()
	unblock()
	receiveRetryResult(t, finished)
	require.ErrorIs(t, task.Wait(), vivid.ErrorActorDeaded, "任务迟到完成不能覆盖停止结果")
}
