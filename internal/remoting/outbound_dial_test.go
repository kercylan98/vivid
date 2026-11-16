package remoting

import (
	"net"
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/require"
)

type dialTestContext struct {
	*retryTestContext
	messages []vivid.Message
}

func (c *dialTestContext) Tell(_ vivid.ActorRef, message vivid.Message) {
	c.messages = append(c.messages, message)
}

func TestEndpoint_WaitingDialDoesNotStartOrRetry(t *testing.T) {
	e, base := newRetryTestEndpoint(t)
	e.endpointManager = dialTestManager(t, 1, 1)
	ctx := &dialTestContext{retryTestContext: base}
	first, _, err := e.endpointManager.requestDial(nil)
	require.NoError(t, err)
	require.True(t, first.begin())
	for range 8 {
		e.ensureSession(ctx)
	}
	require.Empty(t, ctx.tasks)
	require.Zero(t, e.retryCount)
	require.False(t, e.connecting)
	require.Nil(t, e.connectTask)
	require.Equal(t, 1, e.endpointManager.waitDials.Len())
	first.complete()
	require.Len(t, ctx.messages, 1)
	notice := ctx.messages[0].(endpointDialReady)
	e.startDial(ctx, notice.dial)
	e.startDial(ctx, notice.dial)
	require.Len(t, ctx.tasks, 1)
	require.True(t, e.connecting)
	require.Zero(t, e.retryCount)
	e.onKill(ctx)
	require.Equal(t, 1, e.endpointManager.dialCount, "任务未退出前仍保留名额")
	// 执行已取消的任务，真实 DialContext 应立即返回并归还名额。
	message, err := ctx.tasks[0].Run()
	require.NoError(t, err)
	require.Error(t, message.(*endpointConnectCompleted).err)
	require.Zero(t, e.endpointManager.dialCount)
	require.Empty(t, e.endpointManager.pending)
}

func TestEndpoint_InboundCancelsWaitingAndLateDialNotice(t *testing.T) {
	for _, granted := range []bool{false, true} {
		t.Run(map[bool]string{false: "queued", true: "granted"}[granted], func(t *testing.T) {
			e, base := newRetryTestEndpoint(t)
			e.endpointManager = dialTestManager(t, 1, 1)
			e.advertiseAddr = "a:1" // 即使地址偏好出站，等待名额也不应拒绝入站。
			ctx := &dialTestContext{retryTestContext: base}
			first, _, err := e.endpointManager.requestDial(nil)
			require.NoError(t, err)
			require.True(t, first.begin())
			e.ensureSession(ctx)
			old := e.dialRequest
			if granted {
				first.complete()
				require.Len(t, ctx.messages, 1)
			}
			conn, peer := net.Pipe()
			t.Cleanup(func() { _ = conn.Close(); _ = peer.Close() })
			e.onAttachSession(ctx, endpointAttachSession{session: newSession(e.address, e.advertiseAddr, conn, sessionRoleInbound)})
			require.NotNil(t, e.inboundSession)
			require.Nil(t, e.dialRequest)
			require.Equal(t, 0, e.endpointManager.waitDials.Len())
			require.Len(t, ctx.tasks, 1, "仅创建入站响应任务")
			e.startDial(ctx, old)
			require.Len(t, ctx.tasks, 1, "接管后的迟到通知不得拨号")
			if !granted {
				first.complete()
			}
			require.Zero(t, e.endpointManager.dialCount)
		})
	}
}

func TestEndpoint_DialQueueFullStopsWithoutRetry(t *testing.T) {
	e, ctx := newRetryTestEndpoint(t)
	e.endpointManager = dialTestManager(t, 1, 1)
	for range 2 {
		_, _, err := e.endpointManager.requestDial(nil)
		require.NoError(t, err)
	}
	e.ensureSession(ctx)
	require.True(t, e.stopping)
	require.True(t, e.outboundBuffer.Empty())
	require.Equal(t, 1, ctx.killed)
	require.Empty(t, ctx.tasks)
	require.Empty(t, ctx.scheduler.messages, "本地队列满不能进入远端失败重试")
}

func dialTestManager(t *testing.T, active, waiting int) *EndpointManager {
	t.Helper()
	manager := newEndpointManager(vivid.NewActorSystemRemotingOptions(
		vivid.WithActorSystemRemotingMaxConcurrentDials(active),
		vivid.WithActorSystemRemotingMaxPendingDials(waiting),
	))
	t.Cleanup(manager.Stop)
	return manager
}

func TestOutboundDial_DefaultsAndOptions(t *testing.T) {
	for _, options := range []vivid.ActorSystemRemotingOptions{{}, {MaxConcurrentDials: -1, MaxPendingDials: -1}, vivid.NewActorSystemRemotingOptions(
		vivid.WithActorSystemRemotingMaxConcurrentDials(0),
		vivid.WithActorSystemRemotingMaxPendingDials(-1),
	)} {
		manager := newEndpointManager(options)
		require.Equal(t, 64, manager.dialLimit)
		require.Equal(t, 1024, manager.waitLimit)
	}
	manager := dialTestManager(t, 2, 3)
	require.Equal(t, 2, manager.dialLimit)
	require.Equal(t, 3, manager.waitLimit)
}

func TestOutboundDial_FIFOAndCancellation(t *testing.T) {
	manager := dialTestManager(t, 1, 3)
	ready := make(chan *outboundDial, 4)
	notify := func(dial *outboundDial) {
		// 回调可以重新访问管理器，不应在持锁时调用。
		require.False(t, manager.Stopped())
		ready <- dial
	}
	first, immediate, err := manager.requestDial(notify)
	require.NoError(t, err)
	require.True(t, immediate)
	require.True(t, first.begin())
	var waiting []*outboundDial
	for range 3 {
		dial, immediate, err := manager.requestDial(notify)
		require.NoError(t, err)
		require.False(t, immediate)
		require.False(t, dial.begin())
		waiting = append(waiting, dial)
	}
	_, _, err = manager.requestDial(notify)
	require.ErrorIs(t, err, vivid.ErrorRemotingDialQueueFull)
	waiting[1].cancel()
	require.Equal(t, 2, manager.waitDials.Len())
	first.cancel()
	require.Equal(t, 1, manager.dialCount, "执行中的取消必须等待任务退出")
	require.Empty(t, ready)
	first.complete()
	require.Same(t, waiting[0], <-ready)
	require.True(t, waiting[0].begin())
	require.False(t, waiting[0].begin(), "重复通知不能启动第二个任务")
	// 后来的地址不能越过已排队地址。
	last, immediate, err := manager.requestDial(notify)
	require.NoError(t, err)
	require.False(t, immediate)
	waiting[0].complete()
	require.Same(t, waiting[2], <-ready)
	// 通知已发出但尚未开始时取消，应立即归还名额；迟到通知不能启动任务。
	waiting[2].cancel()
	require.False(t, waiting[2].begin())
	require.Same(t, last, <-ready)
	last.cancel()
	require.Empty(t, manager.dials)
	require.Zero(t, manager.dialCount)
	require.Zero(t, manager.waitDials.Len())
	first.complete()
	require.Zero(t, manager.dialCount, "重复结束不能多归还名额")
}

func TestOutboundDial_StopBeforeAndAfterBegin(t *testing.T) {
	manager := dialTestManager(t, 2, 1)
	var notified int
	notify := func(*outboundDial) { notified++ }
	running, _, err := manager.requestDial(notify)
	require.NoError(t, err)
	require.True(t, running.begin())
	granted, _, err := manager.requestDial(notify)
	require.NoError(t, err)
	waiting, immediate, err := manager.requestDial(notify)
	require.NoError(t, err)
	require.False(t, immediate)
	manager.Stop()
	require.False(t, granted.begin())
	require.False(t, waiting.begin())
	require.Equal(t, 1, manager.dialCount)
	require.Len(t, manager.dials, 1)
	running.complete()
	require.Empty(t, manager.dials)
	require.Zero(t, manager.dialCount)
	require.Zero(t, notified)
	_, _, err = manager.requestDial(notify)
	require.ErrorIs(t, err, vivid.ErrorRemotingStopped)
}

func TestOutboundDial_ConcurrentReleaseAndStop(t *testing.T) {
	for range 64 {
		manager := dialTestManager(t, 4, 16)
		ready := make(chan *outboundDial, 16)
		var running []*outboundDial
		for range 4 {
			dial, _, err := manager.requestDial(nil)
			require.NoError(t, err)
			require.True(t, dial.begin())
			running = append(running, dial)
		}
		for range 16 {
			_, immediate, err := manager.requestDial(func(dial *outboundDial) { ready <- dial })
			require.NoError(t, err)
			require.False(t, immediate)
		}
		var group sync.WaitGroup
		for _, dial := range running {
			group.Go(dial.complete)
		}
		group.Go(manager.Stop)
		group.Wait()
		close(ready)
		for dial := range ready {
			require.False(t, dial.begin(), "停止后的迟到名额通知无效")
		}
		require.Empty(t, manager.dials)
		require.Zero(t, manager.waitDials.Len())
		require.Zero(t, manager.dialCount)
	}
}

func BenchmarkOutboundDial_RequestAndComplete(b *testing.B) {
	manager := newEndpointManager()
	b.ReportAllocs()
	for b.Loop() {
		dial, _, _ := manager.requestDial(nil)
		dial.begin()
		dial.complete()
	}
}
