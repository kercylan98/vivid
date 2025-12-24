package remoting

import (
	"fmt"
	"net"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ vivid.Actor = (*serverAcceptActor)(nil)
)

func newServerAcceptActor(serverActor *ServerActor) *serverAcceptActor {
	return &serverAcceptActor{
		listener:       serverActor.listener,
		advertiseAddr:  serverActor.advertiseAddr,
		envelopHandler: serverActor.envelopHandler,
		codec:          serverActor.codec,
	}
}

// serverAcceptActor 是专用于接收远程连接的 Actor
type serverAcceptActor struct {
	listener       net.Listener
	advertiseAddr  string
	envelopHandler NetworkEnvelopHandler
	codec          vivid.Codec
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
	connActor := newTCPConnectionActor(false, conn, a.advertiseAddr, a.codec, a.envelopHandler)
	// 此处存在 GOLAND 误报，必须使用 defer 或匿名函数处理，否则将提示：Potential resource leak: ensure the resource is closed on all execution paths
	func(connActor *tcpConnectionActor) {
		if err = ctx.Ask(ctx.Parent(), connActor).Wait(); err != nil {
			// 连接失败，关闭连接
			if closeErr := connActor.Close(); closeErr != nil {
				ctx.Logger().Warn("close accept connection failed", log.String("advertise_addr", a.advertiseAddr), log.Any("err", fmt.Errorf("%w: %s", err, closeErr)))
			}
		}
	}(connActor)
}
