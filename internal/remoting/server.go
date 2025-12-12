package remoting

import (
	"net"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/sugar"
)

var (
	_ vivid.PrelaunchActor = (*ServerActor)(nil)
)

// NewServerActor 创建新的服务器
func NewServerActor(bindAddr string, advertiseAddr string) *ServerActor {
	return &ServerActor{
		bindAddr:      bindAddr,
		advertiseAddr: advertiseAddr,
	}
}

// ServerActor 管理TCP服务器
type ServerActor struct {
	bindAddr      string
	advertiseAddr string
	listener      net.Listener
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
	// 生命周期如下：
	// OnLaunch -> Listener(Accept) -> Connection -> ReceiveLoop
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		s.onLaunch(ctx)
	case net.Listener:
		s.onAccept(ctx)
	}
}

func (s *ServerActor) onLaunch(ctx vivid.ActorContext) {
	ctx.TellSelf(s.listener)
}

func (s *ServerActor) onAccept(ctx vivid.ActorContext) {
	conn, err := s.listener.Accept()
	if err != nil {
		panic("error accepting connection not impl")
	}

	connActor := newTCPConnectionActor(conn, s.advertiseAddr)
	ctx.ActorOf(connActor, vivid.WithActorName(conn.RemoteAddr().String())).
		Then(func(r sugar.ResultContainer[vivid.ActorRef], ref vivid.ActorRef) *sugar.Result[vivid.ActorRef] {
			return r.Ok(ref)
		}).
		Else(func(r sugar.ResultContainer[vivid.ActorRef], err error) *sugar.Result[vivid.ActorRef] {
			return r.Error(err)
		})
}
