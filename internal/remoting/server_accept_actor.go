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

func newServerAcceptActor(listener net.Listener, advertiseAddr string, envelopHandler NetworkEnvelopHandler, codec vivid.Codec, options vivid.ActorSystemRemotingOptions) *serverAcceptActor {
	return &serverAcceptActor{
		options:        options,
		listener:       listener,
		advertiseAddr:  advertiseAddr,
		envelopHandler: envelopHandler,
		codec:          codec,
	}
}

// serverAcceptActor 是专用于接收远程连接的 Actor
type serverAcceptActor struct {
	options        vivid.ActorSystemRemotingOptions // 远程通信选项
	listener       net.Listener                     // 监听器
	advertiseAddr  string                           // 对外宣称的服务地址
	envelopHandler NetworkEnvelopHandler            // 网络消息处理器
	codec          vivid.Codec                      // 外部跨进程消息编解码器
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

	go func() {
		// 异步握手
		connActor, err := newTCPConnectionActor(false, conn, a.advertiseAddr, a.codec, a.envelopHandler, withTCPConnectionActorReadFailedHandler(a.options.ConnectionReadFailedHandler))
		if err != nil {
			ctx.Logger().Warn("handshake failed", log.String("advertise_addr", a.advertiseAddr), log.Any("err", err))
			return
		}
		ctx.Logger().Debug("handshake success", log.String("advertise_address", a.advertiseAddr))
		if err = ctx.Ask(ctx.Parent(), connActor).Wait(); err != nil {
			// 连接失败，关闭连接
			if closeErr := connActor.Close(); closeErr != nil {
				ctx.Logger().Warn("close accept connection failed", log.String("advertise_addr", a.advertiseAddr), log.Any("err", fmt.Errorf("%w: %s", err, closeErr)))
			}
		}
	}()

	// 进入下一次循环监听
	ctx.TellSelf(a.listener)
}
