package remoting

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpoint_SimultaneousInboundBudget(t *testing.T) {
	manager := newEndpointManager()
	defer manager.Stop()
	conn, peer := net.Pipe()
	defer peer.Close()
	pending := manager.newPendingConnection(0)
	require.True(t, pending.attach(conn))
	inbound := newSession("z:1", "a:1", conn, sessionRoleInbound)
	inbound.pending = pending
	e := &endpoint{address: "z:1", advertiseAddr: "a:1", connecting: true}
	e.onAttachSession(&endpointTestContext{}, endpointAttachSession{session: inbound})
	// 优选拨号未完成前不能关闭反向连接，否则对端可能先将待发消息转为死信。
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(20*time.Millisecond)))
	_, err := peer.Read(make([]byte, 1))
	var timeout net.Error
	require.ErrorAs(t, err, &timeout)
	require.True(t, timeout.Timeout())
	require.True(t, e.connecting)
	require.False(t, e.retryScheduled)
	// 暂存连接继续受原握手预算约束，不因仲裁而取得无限生命周期。
	pending.startTimeout(20 * time.Millisecond)
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(time.Second)))
	_, err = peer.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF)
	require.False(t, pending.release())
}

type endpointTestContext struct{ vivid.ActorContext }

func (*endpointTestContext) Logger() log.Logger {
	return log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))
}

func TestEndpoint_SessionAdmission(t *testing.T) {
	for _, test := range []struct {
		name       string
		local      string
		connecting bool
		active     bool
		closed     bool
		pending    bool
		role       sessionRole
		accept     bool
	}{
		{"inbound_without_dial", "a:1", false, false, false, false, sessionRoleInbound, true},
		{"outbound_without_session", "a:1", true, false, false, false, sessionRoleOutbound, true},
		{"simultaneous_prefer_outbound", "a:1", true, false, false, false, sessionRoleInbound, false},
		{"simultaneous_prefer_inbound", "z:1", true, false, false, false, sessionRoleInbound, true},
		{"active_inbound", "z:1", false, true, false, false, sessionRoleInbound, false},
		{"active_outbound", "a:1", false, true, false, false, sessionRoleOutbound, false},
		{"closed_session", "a:1", false, true, true, false, sessionRoleInbound, true},
		{"pending_inbound", "z:1", false, false, false, true, sessionRoleInbound, false},
		{"pending_outbound", "z:1", false, false, false, true, sessionRoleOutbound, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := &endpoint{address: "m:1", advertiseAddr: test.local, connecting: test.connecting}
			if test.active {
				e.association = &endpointAssociation{session: newSession(e.address, test.local, nil, sessionRoleOutbound)}
				if test.name == "active_outbound" {
					e.association.session.role = sessionRoleInbound
				}
				e.association.session.closed = test.closed
			}
			if test.pending {
				e.inboundSession = newSession(e.address, test.local, nil, sessionRoleInbound)
			}
			next := newSession(e.address, test.local, nil, test.role)
			assert.Equal(t, test.accept, e.shouldAcceptSession(next))
		})
	}
}

func TestEndpoint_StaleConnectCompletion(t *testing.T) {
	for _, stopping := range []bool{false, true} {
		fallback := newSession("m:1", "a:1", nil, sessionRoleInbound)
		e := &endpoint{connectAttempt: 2, connecting: true, retryCount: 1, stopping: stopping, fallbackSession: fallback}
		// 旧拨号错误不得清除新尝试或启动另一轮重试，停止后也不再执行连接逻辑。
		e.onConnectCompleted(nil, &endpointConnectCompleted{attempt: 1, err: errors.New("old dial failed")}, nil)
		assert.True(t, e.connecting)
		assert.Equal(t, 1, e.retryCount)
		assert.False(t, e.retryScheduled)
		assert.Same(t, fallback, e.fallbackSession)
		assert.False(t, fallback.isClosed())
	}
	conn, peer := net.Pipe()
	defer peer.Close()
	stale := newSession("m:1", "a:1", conn, sessionRoleOutbound)
	e := &endpoint{connectAttempt: 2, connecting: true}
	e.onConnectCompleted(nil, &endpointConnectCompleted{attempt: 1, session: stale}, nil)
	require.True(t, stale.isClosed(), "迟到拨号创建的连接必须释放")
	assert.True(t, e.connecting)
}

func TestEndpoint_StaleInboundHandshake(t *testing.T) {
	for _, stopping := range []bool{false, true} {
		conn, peer := net.Pipe()
		stale := newSession("m:1", "a:1", conn, sessionRoleInbound)
		current := newSession("m:1", "a:1", nil, sessionRoleInbound)
		e := &endpoint{inboundSession: current, stopping: stopping}
		e.onInboundReady(nil, &endpointInboundReady{session: stale}, nil)
		assert.True(t, stale.isClosed(), "旧握手结果与停止后的结果只释放连接")
		assert.Same(t, current, e.inboundSession)
		require.NoError(t, peer.Close())
	}
}

func TestEndpoint_InboundHandshakeFailure(t *testing.T) {
	conn, peer := net.Pipe()
	defer peer.Close()
	pending := newSession("m:1", "a:1", conn, sessionRoleInbound)
	e := &endpoint{inboundSession: pending, outboundBuffer: newEndpointOutboundBuffer(endpointBufferPolicy{})}
	e.onInboundReady(&endpointTestContext{}, &endpointInboundReady{session: pending, err: errors.New("handshake failed")}, nil)
	assert.True(t, pending.isClosed())
	assert.Nil(t, e.inboundSession)
	assert.Nil(t, e.association)
	assert.True(t, e.shouldAcceptSession(newSession("m:1", "a:1", nil, sessionRoleInbound)), "握手失败后可重新接受连接")
}
