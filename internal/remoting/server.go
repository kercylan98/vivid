package remoting

import (
	"fmt"
	"net"
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/sugar"
)

var (
	_ vivid.PrelaunchActor = (*ServerActor)(nil)
)

// NewServerActor 创建新的服务器
func NewServerActor(bindAddr string, advertiseAddr string, codec vivid.Codec, envelopHandler NetworkEnvelopHandler) *ServerActor {
	sa := &ServerActor{
		bindAddr:          bindAddr,
		advertiseAddr:     advertiseAddr,
		codec:             codec,
		envelopHandler:    envelopHandler,
		acceptConnections: make(map[string]*tcpConnectionActor),
	}
	sa.remotingMailboxCentralWait.Add(1)
	return sa
}

// ServerActor 管理TCP服务器
type ServerActor struct {
	bindAddr                   string
	advertiseAddr              string
	listener                   net.Listener
	codec                      vivid.Codec
	envelopHandler             NetworkEnvelopHandler
	remotingMailboxCentral     *MailboxCentral
	remotingMailboxCentralWait sync.WaitGroup
	acceptConnections          map[string]*tcpConnectionActor
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
		s.onKill(ctx, message)
	case *vivid.OnKilled:
		s.onKilled(ctx, message)
	}
}

func (s *ServerActor) onLaunch(ctx vivid.ActorContext) {
	s.remotingMailboxCentral = newMailboxCentral(ctx.Ref(), ctx, s.codec)
	s.remotingMailboxCentralWait.Done()

	serverAcceptActor := newServerAcceptActor(s)
	ctx.ActorOf(serverAcceptActor, vivid.WithActorName("acceptor"))
}

func (s *ServerActor) onConnection(ctx vivid.ActorContext, connection *tcpConnectionActor) {
	prefix := "dial"
	if !connection.client {
		prefix = "accept"
		s.acceptConnections[connection.conn.RemoteAddr().String()] = connection
	}
	// 连接至服务端的无需绑定，客户端自行维护连接，不进行复用
	ctx.ActorOf(connection, vivid.WithActorName(fmt.Sprintf("%s-%s", prefix, connection.conn.RemoteAddr().String()))).
		Then(func(rc sugar.ResultContainer[vivid.ActorRef], ar vivid.ActorRef) *sugar.Result[vivid.ActorRef] {
			ctx.Reply(nil)
			return rc.Ok(ar)
		}).
		Else(func(rc sugar.ResultContainer[vivid.ActorRef], err error) *sugar.Result[vivid.ActorRef] {
			return rc.Error(err)
		}).
		Unwrap()
}

func (s *ServerActor) GetRemotingMailboxCentral() *MailboxCentral {
	s.remotingMailboxCentralWait.Wait()
	return s.remotingMailboxCentral
}

func (s *ServerActor) onKill(ctx vivid.ActorContext, _ *vivid.OnKill) {
	s.remotingMailboxCentral.Close()
	for _, actor := range s.acceptConnections {
		if err := actor.Close(); err != nil {
			ctx.Logger().Warn("server accept connect close fail",
				log.String("advertise_addr", actor.advertiseAddr),
				log.Any("err", err),
			)
		}
	}
}

func (s *ServerActor) onKilled(ctx vivid.ActorContext, message *vivid.OnKilled) {
	ctx.Logger().Debug("server actor killed", log.Bool("self", ctx.Ref().Equals(message.Ref)), log.String("target", message.Ref.GetPath()))
}
