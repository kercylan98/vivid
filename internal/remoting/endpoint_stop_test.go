package remoting

import (
	"errors"
	"io"
	"net"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stopTestHandler struct {
	NetworkEnvelopHandler
	failed []vivid.Envelop
	onFail func()
}

func (h *stopTestHandler) HandleFailedRemotingEnvelop(envelop vivid.Envelop) {
	h.failed = append(h.failed, envelop)
	if h.onFail != nil {
		h.onFail()
	}
}

type stopTestContext struct {
	*retryTestContext
	onKill func()
}

func (c *stopTestContext) Kill(vivid.ActorRef, bool, ...string) {
	c.killed++
	if c.onKill != nil {
		c.onKill()
	}
}

func TestEndpoint_StopDetachesBeforeFailureAndKill(t *testing.T) {
	for _, schedulingFailed := range []bool{false, true} {
		t.Run(map[bool]string{false: "exhausted", true: "schedule_failed"}[schedulingFailed], func(t *testing.T) {
			e, base := newRetryTestEndpoint(t)
			e.received = 1
			e.endpointManager.endpoints[e.address] = endpointRegistration{ref: base.ref, dispatched: 1}
			if schedulingFailed {
				base.scheduler.onceErr = errors.New("schedule failed")
			} else {
				e.retryPolicy.limit = 0
			}
			next := &retryTestRef{}
			verifyDetached := func() {
				assert.True(t, e.stopping)
				assert.False(t, base.ref == e.endpointManager.endpoints[e.address].ref)
			}
			ctx := &stopTestContext{retryTestContext: base, onKill: verifyDetached}
			handler := &stopTestHandler{onFail: func() {
				verifyDetached()
				routeContext := &retireTestContext{tell: func(target vivid.ActorRef, _ vivid.Message) { assert.Same(t, next, target) }}
				require.NoError(t, e.endpointManager.emitToAddress(routeContext, e.address, endpointAttachSession{}, func(string) (vivid.ActorRef, error) { return next, nil }))
			}}
			e.envelopHandler = handler
			e.scheduleRetry(ctx, errors.New("connect failed"))
			assert.Equal(t, 1, ctx.killed)
			assert.Len(t, handler.failed, 1)
			assert.Same(t, next, e.endpointManager.endpoints[e.address].ref)
			e.onKill(ctx)
			assert.Same(t, next, e.endpointManager.endpoints[e.address].ref, "旧端点退出不得移除新路由")
		})
	}
}

func TestEndpoint_StopDrainsReservedDeliveries(t *testing.T) {
	e, base := newRetryTestEndpoint(t)
	e.received = 1
	e.endpointManager.endpoints[e.address] = endpointRegistration{ref: base.ref, dispatched: 1}
	e.retryPolicy.limit = 0
	handler := &stopTestHandler{}
	e.envelopHandler = handler
	entered, resume, completed := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	conn, peer := net.Pipe()
	defer peer.Close()
	defer conn.Close()
	pending := e.endpointManager.newPendingConnection(0)
	require.True(t, pending.attach(conn))
	session := newSession(e.address, e.advertiseAddr, conn, sessionRoleInbound)
	session.pending = pending
	var delivered vivid.Message
	router := &retireTestContext{tell: func(_ vivid.ActorRef, message vivid.Message) {
		close(entered)
		<-resume
		delivered = message
	}}
	go func() {
		completed <- e.endpointManager.emitToAddress(router, e.address, endpointAttachSession{session: session}, func(string) (vivid.ActorRef, error) { return nil, errors.New("unexpected creation") })
	}()
	<-entered
	// 一个接入通知尚未入邮箱，另一个业务信封已经排队；两者均须处理后再发送系统终止消息。
	router = &retireTestContext{tell: func(vivid.ActorRef, vivid.Message) {}}
	require.NoError(t, e.endpointManager.emitToAddress(router, e.address, &stopTestEnvelop{}, func(string) (vivid.ActorRef, error) { return nil, errors.New("unexpected creation") }))
	e.scheduleRetry(base, errors.New("dial failed"))
	require.True(t, e.stopping)
	require.Empty(t, e.endpointManager.endpoints)
	require.Zero(t, base.killed, "投递未处理完前不能用系统消息越过普通队列")
	base.message = &stopTestEnvelop{}
	e.OnReceive(base)
	require.Len(t, handler.failed, 2, "缓存消息与终止前已排队的业务都应失败处理")
	require.Zero(t, base.killed)
	close(resume)
	require.NoError(t, <-completed)
	base.message = delivered
	e.OnReceive(base)
	require.Equal(t, 1, base.killed)
	require.True(t, session.isClosed())
	require.Empty(t, e.endpointManager.pending)
	_, err := peer.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF)
	e.finishStop(base)
	require.Equal(t, 1, base.killed, "只能发送一次终止消息")
}

type stopTestEnvelop struct{ vivid.Envelop }
