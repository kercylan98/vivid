package remoting

import (
	"net"

	"github.com/kercylan98/vivid"
)

var (
	_ vivid.Actor = (*serverAcceptActor)(nil)
)

func newServerAcceptActor(serverActor *ServerActor) *serverAcceptActor {
	return &serverAcceptActor{
		listener:       serverActor.listener,
		advertiseAddr:  serverActor.advertiseAddr,
		envelopHandler: serverActor.envelopHandler,
	}
}

// serverAcceptActor 是专用于接收远程连接的 Actor
type serverAcceptActor struct {
	listener       net.Listener
	advertiseAddr  string
	envelopHandler NetworkEnvelopHandler
}

func (a *serverAcceptActor) OnReceive(ctx vivid.ActorContext) {
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.onLaunch(ctx)
	case net.Listener:
		a.onAccept(ctx)
	}
}

func (a *serverAcceptActor) onLaunch(ctx vivid.ActorContext) {
	ctx.TellSelf(a.listener)
}

func (a *serverAcceptActor) onAccept(ctx vivid.ActorContext) {
	conn, err := a.listener.Accept()
	if err != nil {
		panic("error accepting connection not impl")
	}

	// 由 ServerActor 负责管理连接
	connActor := newTCPConnectionActor(false, conn, a.advertiseAddr, a.envelopHandler)
	if err = ctx.Ask(ctx.Parent(), connActor).Wait(); err != nil {
		// 连接失败，关闭连接
		conn.Close()
	}
}
