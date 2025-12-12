package remoting

import (
	"net"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
)

var (
	_ vivid.Actor = (*tcpConnectionActor)(nil)
)

// Connection 表示一个网络连接
type Connection interface {
	vivid.Actor
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	Close() error
}

func newTCPConnectionActor(conn net.Conn, advertiseAddr string) *tcpConnectionActor {
	return &tcpConnectionActor{
		conn:          conn,
		advertiseAddr: advertiseAddr,
	}
}

// tcpConnectionActor TCP连接实现
type tcpConnectionActor struct {
	conn          net.Conn
	advertiseAddr string
}

func (c *tcpConnectionActor) OnReceive(ctx vivid.ActorContext) {
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		c.onLaunch(ctx)
	}
}

func (c *tcpConnectionActor) onLaunch(ctx vivid.ActorContext) {
	handshakeProtocol := &mailbox.Handshake{
		AdvertiseAddr: c.advertiseAddr,
	}

	// 等待客户端握手协议
	if err := handshakeProtocol.Wait(c.conn); err != nil {
		panic(err)
	}

	// 回复服务端握手协议
	if err := handshakeProtocol.Send(c.conn); err != nil {
		panic(err)
	}
}

func (c *tcpConnectionActor) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
