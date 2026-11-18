package remoting

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retryTestRef struct{ vivid.ActorRef }

func (r *retryTestRef) ToActorRefs() vivid.ActorRefs     { return vivid.ActorRefs{r} }
func (r *retryTestRef) Equals(other vivid.ActorRef) bool { return r == other }

type retryTestScheduler struct {
	vivid.Scheduler
	messages  []vivid.Message
	delays    []time.Duration
	onceErr   error
	cancelErr error
}

func (s *retryTestScheduler) Once(_ vivid.ActorRef, delay time.Duration, message vivid.Message, _ ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	s.messages = append(s.messages, message)
	s.delays = append(s.delays, delay)
	return nil, s.onceErr
}

func (s *retryTestScheduler) Cancel(string) error { return s.cancelErr }

type retryTestStream struct{ vivid.EventStream }

func (*retryTestStream) Publish(vivid.EventStreamContext, vivid.Message) {}

type retryTestFuture struct{ vivid.Future[vivid.Message] }

func (*retryTestFuture) PipeTo(vivid.ActorRefs) {}

type retryTestContext struct {
	endpointTestContext
	ref       vivid.ActorRef
	scheduler *retryTestScheduler
	message   vivid.Message
	tasks     []vivid.EntrustTask
	killed    int
}

func (c *retryTestContext) Ref() vivid.ActorRef          { return c.ref }
func (c *retryTestContext) Message() vivid.Message       { return c.message }
func (c *retryTestContext) Scheduler() vivid.Scheduler   { return c.scheduler }
func (*retryTestContext) EventStream() vivid.EventStream { return &retryTestStream{} }
func (*retryTestContext) Metrics() metrics.Metrics       { return metrics.NewDefaultMetrics() }
func (c *retryTestContext) Kill(vivid.ActorRef, bool, ...string) {
	c.killed++
}

func (c *retryTestContext) Entrust(_ time.Duration, task vivid.EntrustTask) vivid.Future[vivid.Message] {
	c.tasks = append(c.tasks, task)
	return &retryTestFuture{}
}

func newRetryTestEndpoint(t *testing.T) (*endpoint, *retryTestContext) {
	t.Helper()
	manager := newEndpointManager()
	t.Cleanup(manager.Stop)
	e := newEndpoint(nil, "peer:1", manager, nil, nil, "local:1", endpointBufferPolicy{}, endpointRetryPolicy{
		limit: 4, initialDelay: time.Second, maxDelay: time.Minute, backoff: 2,
	}, endpointAssociationPolicy{})
	e.outboundBuffer.items = []vivid.Envelop{nil}
	return e, &retryTestContext{ref: &retryTestRef{}, scheduler: &retryTestScheduler{}}
}

func TestEndpoint_RetryNotificationGeneration(t *testing.T) {
	e, ctx := newRetryTestEndpoint(t)
	e.scheduleRetry(ctx, errors.New("first failure"))
	old := ctx.scheduler.messages[0]
	// 定时器已触发时取消可返回未找到，但已入邮箱的消息仍会迟到。
	ctx.scheduler.cancelErr = vivid.ErrorNotFound
	e.cancelRetry(ctx)
	e.scheduleRetry(ctx, errors.New("later failure"))
	current := ctx.scheduler.messages[1]
	require.NotEqual(t, old, current)
	require.Equal(t, 2*time.Second, ctx.scheduler.delays[1])
	for range 8 {
		ctx.message = old
		e.OnReceive(ctx)
	}
	assert.True(t, e.retryScheduled, "旧通知不能清除新退避")
	assert.Equal(t, 2, e.retryCount)
	assert.False(t, e.connecting)
	assert.Empty(t, ctx.tasks)
	ctx.message = current
	e.OnReceive(ctx)
	require.Len(t, ctx.tasks, 1, "当前通知仍应启动连接任务")
	assert.False(t, e.retryScheduled)
	assert.True(t, e.connecting)
	// 即使连接已结束，重复通知也不能再次启动任务或覆盖下一轮定时器。
	e.connectTask.close()
	e.connectTask, e.connecting = nil, false
	e.OnReceive(ctx)
	assert.Len(t, ctx.tasks, 1)
	e.scheduleRetry(ctx, errors.New("next failure"))
	e.OnReceive(ctx)
	assert.True(t, e.retryScheduled)
	assert.Equal(t, 3, e.retryCount)
	assert.Len(t, ctx.tasks, 1)
}

func TestEndpoint_RetryGenerationSurvivesBudgetReset(t *testing.T) {
	e, ctx := newRetryTestEndpoint(t)
	e.scheduleRetry(ctx, errors.New("first failure"))
	old := ctx.scheduler.messages[0]
	e.cancelRetry(ctx)
	e.failPending(ctx)
	require.Zero(t, e.retryCount)
	e.outboundBuffer.items = []vivid.Envelop{nil}
	e.scheduleRetry(ctx, errors.New("new budget"))
	require.Equal(t, time.Second, ctx.scheduler.delays[1])
	require.NotEqual(t, old, ctx.scheduler.messages[1], "重试次数重置不能复用旧通知身份")
	ctx.message = old
	e.OnReceive(ctx)
	assert.True(t, e.retryScheduled)
	assert.Equal(t, 1, e.retryCount)
	assert.Empty(t, ctx.tasks)
	ctx.message = ctx.scheduler.messages[1]
	e.OnReceive(ctx)
	assert.Len(t, ctx.tasks, 1)
}

func TestEndpoint_RetryNotificationAfterCancellation(t *testing.T) {
	for _, action := range []string{"cancel", "cancel_error", "stop", "schedule_error", "inbound"} {
		t.Run(action, func(t *testing.T) {
			e, ctx := newRetryTestEndpoint(t)
			if action == "schedule_error" {
				ctx.scheduler.onceErr = errors.New("schedule failed")
			}
			e.scheduleRetry(ctx, errors.New("failure"))
			switch action {
			case "cancel_error":
				ctx.scheduler.cancelErr = errors.New("cancel failed")
				e.cancelRetry(ctx)
			case "cancel":
				e.cancelRetry(ctx)
			case "stop":
				e.onKill(ctx)
			case "inbound":
				conn, peer := net.Pipe()
				t.Cleanup(func() { _ = conn.Close(); _ = peer.Close() })
				e.onAttachSession(ctx, endpointAttachSession{session: newSession(e.address, e.advertiseAddr, conn, sessionRoleInbound)})
				require.NotNil(t, e.inboundSession)
			}
			if action == "schedule_error" {
				require.Equal(t, 1, ctx.killed, "调度失败应结束端点并处理待发消息")
				require.True(t, e.outboundBuffer.Empty())
			}
			require.False(t, e.retryScheduled)
			count, attempt, tasks, killed := e.retryCount, e.retryAttempt, len(ctx.tasks), ctx.killed
			for range 8 {
				ctx.message = ctx.scheduler.messages[0]
				e.OnReceive(ctx)
			}
			assert.False(t, e.retryScheduled)
			assert.Equal(t, count, e.retryCount)
			assert.Equal(t, attempt, e.retryAttempt)
			assert.Len(t, ctx.tasks, tasks, "取消后迟到的通知不能启动任务")
			assert.Equal(t, killed, ctx.killed)
			assert.False(t, e.connecting)
		})
	}
}
