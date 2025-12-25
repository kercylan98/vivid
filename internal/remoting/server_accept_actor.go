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

func newServerAcceptActor(listener net.Listener, advertiseAddr string, envelopHandler NetworkEnvelopHandler, codec vivid.Codec) *serverAcceptActor {
	return &serverAcceptActor{
		listener:       listener,
		advertiseAddr:  advertiseAddr,
		envelopHandler: envelopHandler,
		codec:          codec,
	}
}

// serverAcceptActor 是专用于接收远程连接的 Actor
type serverAcceptActor struct {
	listener       net.Listener          // 监听器
	advertiseAddr  string                // 对外宣称的服务地址
	envelopHandler NetworkEnvelopHandler // 网络消息处理器
	codec          vivid.Codec           // 外部跨进程消息编解码器
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
		// 监听失败，可能为主动关闭或系统异常，销毁 Actor 并退出
		ctx.Kill(ctx.Ref(), false, fmt.Sprintf("server listener accept connection failed: %s", err))
		return
	}

	// 进入下一次循环监听
	defer func(listener net.Listener) {
		ctx.TellSelf(listener)
	}(a.listener)

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
