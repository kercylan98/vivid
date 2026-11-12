package remoting

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
