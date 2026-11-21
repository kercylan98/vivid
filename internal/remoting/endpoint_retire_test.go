package remoting

import (
	"errors"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retireTestRef struct{ vivid.ActorRef }

func (r *retireTestRef) Equals(other vivid.ActorRef) bool { return r == other }

type retireTestContext struct {
	endpointTestContext
	ref  vivid.ActorRef
	tell func(vivid.ActorRef, vivid.Message)
	kill func(vivid.ActorRef)
}

func (c *retireTestContext) Ref() vivid.ActorRef { return c.ref }

func (c *retireTestContext) Tell(ref vivid.ActorRef, message vivid.Message) { c.tell(ref, message) }

func (c *retireTestContext) Kill(ref vivid.ActorRef, _ bool, _ ...string) { c.kill(ref) }

func TestEndpoint_RetireOnlyWhenIdle(t *testing.T) {
	for _, field := range []string{"idle", "association", "inbound", "fallback", "connecting", "task", "dial", "retry", "writing", "buffer", "queued", "unstarted", "replaced", "stopping", "manager_stopped"} {
		t.Run(field, func(t *testing.T) {
			manager := newEndpointManager()
			defer manager.Stop()
			ref := &retireTestRef{}
			manager.endpoints["peer:1"] = endpointRegistration{ref: ref, dispatched: 1}
			e := &endpoint{address: "peer:1", endpointManager: manager, outboundBuffer: newEndpointOutboundBuffer(endpointBufferPolicy{}), received: 1}
			switch field {
			case "association":
				e.association = &endpointAssociation{}
			case "inbound":
				e.inboundSession = &session{}
			case "fallback":
				e.fallbackSession = &session{}
			case "connecting":
				e.connecting = true
			case "task":
				e.connectTask = manager.newPendingConnection(0)
			case "dial":
				e.dialRequest, _, _ = manager.requestDial(nil)
			case "retry":
				e.retryScheduled = true
			case "writing":
				e.writing = true
			case "buffer":
				e.outboundBuffer.items = []vivid.Envelop{nil}
			case "queued":
				manager.endpoints[e.address] = endpointRegistration{ref: ref, dispatched: 2}
			case "unstarted":
				e.received = 0
				manager.endpoints[e.address] = endpointRegistration{ref: ref}
			case "replaced":
				manager.endpoints[e.address] = endpointRegistration{ref: &retireTestRef{}, dispatched: 1}
			case "stopping":
				e.stopping = true
			case "manager_stopped":
				manager.Stop()
			}
			killed := 0
			ctx := &retireTestContext{ref: ref, kill: func(target vivid.ActorRef) { assert.Same(t, ref, target); killed++ }}
			e.retireIfIdle(ctx)
			if field == "idle" {
				assert.Equal(t, 1, killed)
				assert.True(t, e.stopping)
				assert.Empty(t, manager.endpoints)
				e.retireIfIdle(ctx)
				assert.Equal(t, 1, killed, "回收只能执行一次")
			} else {
				assert.Zero(t, killed)
				assert.Len(t, manager.endpoints, 1)
			}
		})
	}
}

func TestEndpoint_RetireWaitsForEnqueue(t *testing.T) {
	manager := newEndpointManager()
	defer manager.Stop()
	ref := &retireTestRef{}
	manager.endpoints["peer:1"] = endpointRegistration{ref: ref, dispatched: 1}
	entered, resume := make(chan struct{}), make(chan struct{})
	completed := make(chan error, 1)
	ctx := &retireTestContext{ref: ref, tell: func(target vivid.ActorRef, _ vivid.Message) {
		assert.Same(t, ref, target)
		close(entered)
		<-resume
	}}
	go func() {
		completed <- manager.emitToAddress(ctx, "peer:1", endpointAttachSession{}, func(string) (vivid.ActorRef, error) {
			return nil, errors.New("不应创建新端点")
		})
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("未进入投递窗口")
	}
	// 投递已保留但还未入邮箱，回收不得越过该窗口；Tell 中也没有持有索引锁。
	assert.False(t, manager.retireEndpoint("peer:1", ref, 1))
	close(resume)
	require.NoError(t, <-completed)
	assert.True(t, manager.retireEndpoint("peer:1", ref, 2))
}

func TestEndpoint_NewRouteBeforeOldKill(t *testing.T) {
	manager := newEndpointManager()
	defer manager.Stop()
	old, next := &retireTestRef{}, &retireTestRef{}
	manager.endpoints["peer:1"] = endpointRegistration{ref: old, dispatched: 1}
	e := &endpoint{address: "peer:1", endpointManager: manager, outboundBuffer: newEndpointOutboundBuffer(endpointBufferPolicy{}), received: 1}
	entered, resume, completed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	ctx := &retireTestContext{ref: old, kill: func(vivid.ActorRef) { close(entered); <-resume }}
	go func() { e.retireIfIdle(ctx); close(completed) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("未进入旧端点终止窗口")
	}
	ctx = &retireTestContext{tell: func(target vivid.ActorRef, _ vivid.Message) { assert.Same(t, next, target) }}
	require.NoError(t, manager.emitToAddress(ctx, "peer:1", endpointAttachSession{}, func(string) (vivid.ActorRef, error) { return next, nil }))
	manager.unregisterEndpoint("peer:1", old)
	assert.Same(t, next, manager.endpoints["peer:1"].ref, "旧实例终止不能删除新实例路由")
	assert.False(t, manager.retireEndpoint("peer:1", old, 1))
	close(resume)
	<-completed
	assert.True(t, manager.retireEndpoint("peer:1", next, 1))
}

func TestEndpoint_ConcurrentCreationAndStop(t *testing.T) {
	manager := newEndpointManager()
	defer manager.Stop()
	started, resume := make(chan struct{}, 2), make(chan struct{})
	delivered, killed := make(chan vivid.ActorRef, 2), make(chan vivid.ActorRef, 2)
	ctx := &retireTestContext{tell: func(ref vivid.ActorRef, _ vivid.Message) { delivered <- ref }, kill: func(ref vivid.ActorRef) { killed <- ref }}
	completed := make(chan error, 2)
	for range 2 {
		go func() {
			completed <- manager.emitToAddress(ctx, "peer:1", endpointAttachSession{}, func(string) (vivid.ActorRef, error) {
				started <- struct{}{}
				<-resume
				return &retireTestRef{}, nil
			})
		}()
	}
	for range 2 {
		<-started
	}
	close(resume)
	for range 2 {
		require.NoError(t, <-completed)
	}
	first := <-delivered
	assert.Same(t, first, <-delivered, "并行创建只保留一个接收端点")
	assert.NotSame(t, first, <-killed)
	assert.False(t, manager.retireEndpoint("peer:1", first, 1))
	assert.True(t, manager.retireEndpoint("peer:1", first, 2))
	// 创建期间停止时，不登记或投递迟到的新端点。
	err := manager.emitToAddress(ctx, "peer:2", endpointAttachSession{}, func(string) (vivid.ActorRef, error) {
		manager.Stop()
		return &retireTestRef{}, nil
	})
	assert.ErrorIs(t, err, vivid.ErrorRemotingStopped)
	assert.Empty(t, manager.endpoints)
	assert.Empty(t, delivered)
	assert.Len(t, killed, 1)
}
