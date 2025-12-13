package remoting

import (
	"fmt"
	"net"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/sugar"
)

var (
	_ vivid.PrelaunchActor = (*ServerActor)(nil)
)

// NewServerActor 创建新的服务器
func NewServerActor(bindAddr string, advertiseAddr string, envelopHandler NetworkEnvelopHandler) *ServerActor {
	return &ServerActor{
		bindAddr:       bindAddr,
		advertiseAddr:  advertiseAddr,
		envelopHandler: envelopHandler,
	}
}

// ServerActor 管理TCP服务器
type ServerActor struct {
	bindAddr               string
	advertiseAddr          string
	listener               net.Listener
	envelopHandler         NetworkEnvelopHandler
	remotingMailboxCentral *MailboxCentral
}

func (s *ServerActor) OnPrelaunch() error {
	addr, err := net.ResolveTCPAddr("tcp", s.bindAddr)
	if err != nil {
		return err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = listener
	return nil
}

func (s *ServerActor) OnReceive(ctx vivid.ActorContext) {
	switch message := ctx.Message().(type) {
	case *vivid.OnLaunch:
		s.onLaunch(ctx)
	case *tcpConnectionActor:
		s.onConnection(ctx, message)
	case *vivid.OnKill:
		s.remotingMailboxCentral.Close()
	}
}

func (s *ServerActor) onLaunch(ctx vivid.ActorContext) {
	s.remotingMailboxCentral = newMailboxCentral(ctx.Ref(), ctx)

	serverAcceptActor := newServerAcceptActor(s)
	ctx.ActorOf(serverAcceptActor, vivid.WithActorName("acceptor"))
}

func (s *ServerActor) onConnection(ctx vivid.ActorContext, connection *tcpConnectionActor) {
	prefix := "accept"
	if connection.client {
		prefix = "dial"
	}

	// 连接至服务端的无需绑定，客户端自行维护连接，不进行复用
	ctx.ActorOf(connection, vivid.WithActorName(fmt.Sprintf("%s-%s", prefix, connection.conn.RemoteAddr().String()))).
		Then(func(rc sugar.ResultContainer[vivid.ActorRef], ar vivid.ActorRef) *sugar.Result[vivid.ActorRef] {
			ctx.Reply(nil)
			return rc.Ok(ar)
		}).
		Else(func(rc sugar.ResultContainer[vivid.ActorRef], err error) *sugar.Result[vivid.ActorRef] {
			ctx.Reply(err)
			return rc.None()
		})
}

func (s *ServerActor) GetRemotingMailboxCentral() *MailboxCentral {
	return s.remotingMailboxCentral
}
