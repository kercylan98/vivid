package remoting

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPendingConnection_StopAndLateAttach(t *testing.T) {
	manager := newEndpointManager()
	first := manager.newPendingConnection(time.Second)
	conn, peer := net.Pipe()
	defer peer.Close()
	require.True(t, first.attach(conn))
	manager.Stop()
	_, err := peer.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF)
	require.Error(t, first.ctx.Err())
	require.False(t, first.release())
	// Stop 后才返回的 Accept 或 Dial 结果也必须关闭。
	late := manager.newPendingConnection(0)
	conn, other := net.Pipe()
	defer other.Close()
	require.False(t, late.attach(conn))
	_, err = other.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF)
}

func TestPendingConnection_TimeoutWithoutResultDelivery(t *testing.T) {
	manager := newEndpointManager()
	defer manager.Stop()
	pending := manager.newPendingConnection(20 * time.Millisecond)
	conn, peer := net.Pipe()
	defer peer.Close()
	require.True(t, pending.attach(conn))
	// 不投递任何完成消息，模拟任务完成前 Actor 已被终止。
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(time.Second)))
	_, err := peer.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF)
	require.False(t, pending.release())
}

func TestPendingConnection_ReleaseAndStop(t *testing.T) {
	for range 64 {
		manager := newEndpointManager()
		pending := manager.newPendingConnection(time.Second)
		conn, peer := net.Pipe()
		require.True(t, pending.attach(conn))
		var wait sync.WaitGroup
		wait.Add(1)
		go func() {
			defer wait.Done()
			manager.Stop()
		}()
		released := pending.release()
		wait.Wait()
		if released {
			// 已交接会话不受任务取消影响，仍能收发数据。
			go func() { _, _ = conn.Write([]byte{42}) }()
			require.NoError(t, peer.SetReadDeadline(time.Now().Add(time.Second)))
			data := make([]byte, 1)
			_, err := io.ReadFull(peer, data)
			require.NoError(t, err)
			require.Equal(t, byte(42), data[0])
		} else {
			_, err := peer.Read(make([]byte, 1))
			require.ErrorIs(t, err, io.EOF)
		}
		_ = conn.Close()
		_ = peer.Close()
	}
}

func TestPendingConnection_ReleaseBeforeTimeout(t *testing.T) {
	manager := newEndpointManager()
	defer manager.Stop()
	pending := manager.newPendingConnection(20 * time.Millisecond)
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	require.True(t, pending.attach(conn))
	require.True(t, pending.release())
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(40*time.Millisecond)))
	_, err := peer.Read(make([]byte, 1))
	var timeout net.Error
	require.ErrorAs(t, err, &timeout)
	require.True(t, timeout.Timeout(), "成功交接后的连接不能再被原握手计时器关闭")
}

type blockedWriteConn struct {
	net.Conn
	started chan struct{}
	once    sync.Once
}

func (c *blockedWriteConn) Write(data []byte) (int, error) {
	c.once.Do(func() { close(c.started) })
	return c.Conn.Write(data)
}

func TestSession_GracefulCloseWithBlockedWriter(t *testing.T) {
	conn, peer := net.Pipe()
	defer peer.Close()
	blocked := &blockedWriteConn{Conn: conn, started: make(chan struct{})}
	session := newSession("peer", "local", blocked, sessionRoleOutbound)
	defer session.Close()
	written := make(chan error, 1)
	go func() { written <- session.Write([]byte{1}) }()
	<-blocked.started
	closed := make(chan struct{})
	go func() {
		closeEndpointSession(&endpointTestContext{}, "peer", session, true, "test")
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("关闭控制帧不能无限等待业务写锁")
	}
	require.Error(t, <-written)
	require.True(t, session.isClosed())
	_, err := peer.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF)
}
