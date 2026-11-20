package actor

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestSystem_StopPausedTree(t *testing.T) {
	system := NewTestSystem(t,
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemRemoting(admissionSystemAddress(t)),
		vivid.WithActorSystemEnableMetrics(true),
	)
	var contexts []*Context
	t.Cleanup(func() {
		for _, ctx := range contexts {
			ctx.mailbox.Resume()
		}
		_ = system.Stop(time.Second)
		receiveRetryResult(t, system.rootCleanupCompleted)
	})
	launched := make(chan *Context, 2)
	var messages, kills atomic.Int32
	childBehavior := vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			launched <- ctx.(*Context)
		case *vivid.OnKill:
			kills.Add(1)
		case string:
			messages.Add(1)
		}
	})
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			if _, err := ctx.ActorOf(childBehavior); err != nil {
				panic(err)
			}
			launched <- ctx.(*Context)
		case *vivid.OnKill:
			kills.Add(1)
		case string:
			messages.Add(1)
		}
	}))
	require.NoError(t, err)
	for range 2 {
		ctx := receiveRetryResult(t, launched)
		ctx.mailbox.Pause()
		system.Tell(ctx.ref, "queued business message")
	}
	contexts = append(contexts, system.Context)
	system.actorContexts.Range(func(_, value any) bool {
		if ctx, ok := value.(*Context); ok {
			contexts = append(contexts, ctx)
		}
		return true
	})
	for _, ctx := range contexts {
		ctx.mailbox.Pause()
	}
	require.NoError(t, system.Stop(time.Second), "暂停普通邮箱不能阻断系统关闭")
	require.EqualValues(t, 2, kills.Load())
	require.Zero(t, messages.Load(), "停止不能恢复消费暂停的业务消息")
	listener, err := net.Listen("tcp", system.Ref().GetAddress())
	require.NoError(t, err, "完成停止后必须释放监听端口")
	require.NoError(t, listener.Close())
}

func TestSystem_StopTimeoutContinues(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() {
		once.Do(func() { close(release) })
		_ = system.Stop(time.Second)
		receiveRetryResult(t, system.rootCleanupCompleted)
	})
	var kills atomic.Int32
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnKill); ok {
			kills.Add(1)
			close(entered)
			<-release
		}
	}))
	require.NoError(t, err)
	first := make(chan error, 1)
	go func() { first <- system.Stop(20 * time.Millisecond) }()
	receiveRetryResult(t, entered)
	err = receiveRetryResult(t, first)
	require.ErrorIs(t, err, vivid.ErrorActorSystemStopFailed)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NoError(t, system.options.Context.Err(), "等待超时不能提前取消尚在执行的有序关闭")
	_, err = system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
	require.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
	_, err = system.ClusterReady().Result()
	require.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
	require.ErrorIs(t, system.Start(), vivid.ErrorActorSystemAlreadyStopped)

	// 并发等待者各自超时，不重复投递停止，也不将停止中误报为已经停止。
	var group sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		group.Go(func() { results <- system.Stop(20 * time.Millisecond) })
	}
	group.Wait()
	for range 8 {
		require.ErrorIs(t, <-results, vivid.ErrorActorSystemStopFailed)
	}
	second := make(chan error, 1)
	go func() { second <- system.Stop(0) }()
	select {
	case err := <-second:
		t.Fatalf("停止尚未完成，不得提前返回：%v", err)
	case <-time.After(20 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	require.NoError(t, receiveRetryResult(t, second))
	require.ErrorIs(t, system.options.Context.Err(), context.Canceled)
	require.EqualValues(t, 1, kills.Load())
	require.ErrorIs(t, system.Stop(), vivid.ErrorActorSystemAlreadyStopped)
}

func TestSystem_StopWaitsForInFlightSpawn(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() {
		once.Do(func() { close(release) })
		err := system.Stop(time.Second)
		require.True(t, err == nil || errors.Is(err, vivid.ErrorActorSystemAlreadyStopped), "%v", err)
	})
	spawned := make(chan error, 1)
	var killed atomic.Bool
	go func() {
		_, err := system.ActorOf(vivid.NewPrelaunchActor(func(vivid.PrelaunchContext) error {
			close(entered)
			<-release
			return nil
		}, vivid.ActorFN(func(ctx vivid.ActorContext) {
			if _, ok := ctx.Message().(*vivid.OnKill); ok {
				killed.Store(true)
			}
		})))
		spawned <- err
	}()
	receiveRetryResult(t, entered)
	require.ErrorIs(t, system.Stop(20*time.Millisecond), vivid.ErrorActorSystemStopFailed, "预启动回调不能突破调用方等待预算")
	once.Do(func() { close(release) })
	require.NoError(t, receiveRetryResult(t, spawned))
	err := system.Stop(time.Second)
	require.True(t, err == nil || errors.Is(err, vivid.ErrorActorSystemAlreadyStopped), "%v", err)
	require.True(t, killed.Load(), "停止前已开始创建的 Actor 必须纳入关闭")
}

func TestSystem_StopFromParentContext(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	system := NewTestSystem(t,
		vivid.WithActorSystemContext(parent),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
	)
	launched := make(chan *Context, 1)
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
			launched <- ctx.(*Context)
		}
	}))
	require.NoError(t, err)
	ctx := receiveRetryResult(t, launched)
	ctx.mailbox.Pause()
	system.mailbox.Pause()
	t.Cleanup(func() { ctx.mailbox.Resume(); system.mailbox.Resume(); _ = system.Stop(time.Second) })
	cancel()
	receiveRetryResult(t, system.stopCompleted)
	require.ErrorIs(t, system.options.Context.Err(), context.Canceled)
	require.ErrorIs(t, system.Stop(), vivid.ErrorActorSystemAlreadyStopped)
}
