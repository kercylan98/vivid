package remoting

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var invalidHandshakeAddresses = []string{
	"", "node.example", ":8080", "0.0.0.0:8080", "[::]:8080",
	"[0:0:0:0:0:0:0:0]:8080", "[::ffff:0.0.0.0]:8080",
	"node:0", "node:65536", "node:+80", "node:http", " node:8080 ",
}

func TestHandshake_WaitAddress(t *testing.T) {
	addresses := append([]string{"127.0.0.1:1", "node.example:65535", "[::1]:8080", "[::ffff:127.0.0.1]:8080"}, invalidHandshakeAddresses...)
	for index, address := range addresses {
		t.Run(address, func(t *testing.T) {
			conn, peer := net.Pipe()
			defer conn.Close()
			defer peer.Close()
			sent := make(chan error, 1)
			go func() {
				_, err := NewHandshakeFrame(address).WriteTo(peer)
				sent <- err
			}()
			handshake := &Handshake{}
			err := handshake.Wait(conn)
			require.NoError(t, <-sent)
			if index < 4 {
				require.NoError(t, err)
				assert.Equal(t, address, handshake.AdvertiseAddr)
			} else {
				require.ErrorIs(t, err, vivid.ErrorRemotingHandshake)
				assert.Empty(t, handshake.AdvertiseAddr)
			}
		})
	}
}

type handshakeTestContext struct {
	endpointTestContext
	acceptRequests int
}

func (*handshakeTestContext) Ref() vivid.ActorRef { return nil }

func (c *handshakeTestContext) Tell(_ vivid.ActorRef, message vivid.Message) {
	if _, ok := message.(acceptConnection); ok {
		c.acceptRequests++
	}
}

func TestAcceptor_InvalidHandshakeCleanup(t *testing.T) {
	for _, address := range invalidHandshakeAddresses {
		for _, late := range []bool{false, true} {
			t.Run(address+"/late="+map[bool]string{true: "true", false: "false"}[late], func(t *testing.T) {
				manager := newEndpointManager()
				defer manager.Stop()
				conn, peer := net.Pipe()
				defer peer.Close()
				task := manager.newPendingConnection(0)
				require.True(t, task.attach(conn))
				a := &acceptor{endpointManager: manager, pendingHandshakes: map[uint64]*pendingConnection{1: task}}
				ctx := &handshakeTestContext{}
				sent := make(chan error, 1)
				go func() {
					_, err := NewHandshakeFrame(address).WriteTo(peer)
					sent <- err
				}()
				peerAddr, err := a.doHandshake(conn)
				require.NoError(t, <-sent)
				require.ErrorIs(t, err, vivid.ErrorRemotingHandshake)
				if late {
					a.onKill(ctx)
				}
				a.onHandshakeCompleted(ctx, &acceptHandshakeCompleted{seq: 1, conn: conn, peerAddr: peerAddr, err: err}, nil)
				assert.Empty(t, a.pendingHandshakes)
				assert.Empty(t, manager.pending)
				assert.Empty(t, manager.endpoints, "无效握手不能创建端点身份")
				assert.Equal(t, map[bool]int{true: 0, false: 1}[late], ctx.acceptRequests, "未停止时须继续接入，停止后不再接入")
				assert.Error(t, task.ctx.Err())
				_, err = peer.Read(make([]byte, 1))
				assert.ErrorIs(t, err, io.EOF, "错误返回后不保留连接")
			})
		}
	}
}

func TestAcceptor_HandshakeAdmission(t *testing.T) {
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	sent := make(chan error, 1)
	go func() { sent <- (&Handshake{AdvertiseAddr: "peer:1234"}).Send(peer) }()
	a := &acceptor{advertiseAddr: "local:1234"}
	address, err := a.doHandshake(conn)
	require.NoError(t, err)
	require.NoError(t, <-sent)
	assert.Equal(t, "peer:1234", address)
	// 接收请求不代表接受会话；Endpoint 仲裁前，对端不得收到可发送数据的确认。
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(20*time.Millisecond)))
	_, err = ReadFrame(peer, nil, FrameReadLimits{MaxDataLen: 4096})
	var timeout net.Error
	require.ErrorAs(t, err, &timeout)
	assert.True(t, timeout.Timeout())
}
