package actor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func requireStoppedRequest(t *testing.T, request interface {
	Wait() error
	Close(error)
}) {
	t.Helper()
	defer request.Close(context.Canceled)
	completed := make(chan error, 1)
	go func() { completed <- request.Wait() }()
	require.ErrorIs(t, receiveRetryResult(t, completed), vivid.ErrorActorSystemStopped)
}

func TestSystem_RequestsAfterStop(t *testing.T) {
	var activations atomic.Int32
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			activations.Add(1)
			return vivid.ActorFN(func(ctx vivid.ActorContext) {})
		})),
	)
	local, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
	require.NoError(t, err)
	remote, err := system.CreateRef("127.0.0.1:1", "/peer")
	require.NoError(t, err)
	virtual := system.VirtualRef("worker", "one")
	require.NoError(t, system.Stop(time.Second))
	var tasks atomic.Int32
	for _, request := range []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "local ask", run: func(t *testing.T) { requireStoppedRequest(t, system.Ask(local, "request", 0)) }},
		{name: "remote ask", run: func(t *testing.T) { requireStoppedRequest(t, system.Ask(remote, "request", 0)) }},
		{name: "virtual ask", run: func(t *testing.T) { requireStoppedRequest(t, system.Ask(virtual, "request", 0)) }},
		{name: "explicit sender", run: func(t *testing.T) { requireStoppedRequest(t, system.AskWithSender(system.Ref(), local, "request", 0)) }},
		{name: "local probe", run: func(t *testing.T) { requireStoppedRequest(t, system.Probe(local, 0)) }},
		{name: "remote probe", run: func(t *testing.T) { requireStoppedRequest(t, system.Probe(remote, 0)) }},
		{name: "entrust", run: func(t *testing.T) {
			requireStoppedRequest(t, system.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
				tasks.Add(1)
				return "unexpected", nil
			})))
		}},
	} {
		t.Run(request.name, request.run)
	}
	require.Zero(t, activations.Load(), "停止后不能进入虚拟激活")
	require.Zero(t, tasks.Load(), "被拒绝的托管任务不能启动")
	system.futureLock.RLock()
	require.Empty(t, system.futureAgents)
	system.futureLock.RUnlock()
	retained := 0
	system.actorContexts.Range(func(_, value any) bool {
		if _, ok := value.(*future.Future[vivid.Message]); ok {
			retained++
		}
		return true
	})
	require.Zero(t, retained, "停止后不能保留新的等待路由")
}

func TestSystem_RequestRegistrationAndStop(t *testing.T) {
	for range 200 {
		system := &System{futureAgents: make(map[vivid.ActorPath]map[vivid.ActorPath]*AgentRef)}
		agent, request, cleaned := newRegistrationFuture(t, system, 0)
		var workers sync.WaitGroup
		start := make(chan struct{})
		workers.Add(2)
		go func() {
			defer workers.Done()
			<-start
			if err := system.appendFuture(agent, request, nil, 0); err != nil {
				request.Close(err)
			}
		}()
		go func() {
			defer workers.Done()
			<-start
			system.closeFutureRequests()
		}()
		close(start)
		workers.Wait()
		receiveRetryResult(t, cleaned)
		require.ErrorIs(t, request.Wait(), vivid.ErrorActorSystemStopped)
		requireFutureUnregistered(t, system, agent)
	}
}

func TestSystem_StopClosesLateRootRequest(t *testing.T) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() {
		if err := system.Stop(time.Second); err != nil {
			require.ErrorIs(t, err, vivid.ErrorActorSystemAlreadyStopped)
		}
	})
	var local vivid.ActorRef
	pending := make(chan vivid.Future[vivid.Message], 1)
	system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
		if event, ok := event.(ves.ActorKilledEvent); ok && event.ActorRef.Equals(system.Ref()) {
			// 根 Actor 已结束收尾，迟到请求应由其生命周期直接拒绝。
			pending <- system.Ask(local, "late root request", 0)
		}
	}}
	require.NoError(t, system.Start())
	var err error
	local, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
	require.NoError(t, err)
	require.NoError(t, system.Stop(time.Second))
	require.ErrorIs(t, receiveRetryResult(t, pending).Wait(), vivid.ErrorActorDeaded)
	system.futureLock.RLock()
	require.Empty(t, system.futureAgents)
	system.futureLock.RUnlock()
}

func TestSystem_StopAllowsShutdownRequests(t *testing.T) {
	newSystem := func() *System {
		system := NewTestSystem(t,
			vivid.WithActorSystemRemoting(admissionSystemAddress(t)),
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		)
		t.Cleanup(func() {
			if err := system.Stop(3 * time.Second); err != nil {
				require.ErrorIs(t, err, vivid.ErrorActorSystemAlreadyStopped)
			}
		})
		return system.System
	}
	peer, system := newSystem(), newSystem()
	target, err := peer.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(vivid.ActorRef); ok {
			ctx.Reply(message)
		}
	}))
	require.NoError(t, err)
	completed := make(chan error, 1)
	var tasks atomic.Int32
	_, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnKill); !ok {
			return
		}
		_, err := ctx.Ask(target, ctx.Ref(), 2*time.Second).Result()
		if err == nil {
			err = ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
				tasks.Add(1)
				return "shutdown task", nil
			})).Wait()
		}
		completed <- err
	}))
	require.NoError(t, err)
	require.NoError(t, system.Stop(3*time.Second), "内部收尾期间仍需保留请求能力")
	require.NoError(t, receiveRetryResult(t, completed))
	require.EqualValues(t, 1, tasks.Load())
}
