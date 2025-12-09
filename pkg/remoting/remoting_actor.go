package remoting

import (
	"net"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/pkg/result"
)

var (
	_ vivid.RemotingActor = (*RemotingActor)(nil)
)

func NewRemotingActor(network, bindAddress, advertiseAddress string) *result.Result[*RemotingActor] {
	bindAddr, err := utils.ResolveNetAddr(network, bindAddress)
	if err != nil {
		return result.Error[*RemotingActor](err)
	}
	advertiseAddr, err := utils.ResolveNetAddr(network, advertiseAddress)
	if err != nil {
		return result.Error[*RemotingActor](err)
	}
	return result.With(&RemotingActor{
		bindAddress:      bindAddr,
		advertiseAddress: advertiseAddr,
	}, nil)
}

type RemotingActor struct {
	bindAddress      net.Addr
	advertiseAddress net.Addr
}

// GetAdvertiseAddress implements vivid.RemotingActor.
func (r *RemotingActor) GetAdvertiseAddress() net.Addr {
	return r.advertiseAddress
}

// GetBindAddress implements vivid.RemotingActor.
func (r *RemotingActor) GetBindAddress() net.Addr {
	return r.bindAddress
}

func (r *RemotingActor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		r.onLaunch(ctx)
	case net.Listener:
		r.onListener(ctx, msg)
	}
}

func (r *RemotingActor) onLaunch(ctx vivid.ActorContext) {
	listener, err := net.Listen(r.bindAddress.Network(), r.bindAddress.String())
	if err != nil {
		ctx.Logger().Error("failed to listen", "error", err)
		return
	}

	ctx.Tell(ctx.Ref(), listener)
}

func (r *RemotingActor) onListener(ctx vivid.ActorContext, listener net.Listener) {
	conn, err := listener.Accept()
	if err != nil {
		ctx.Logger().Error("failed to accept", "error", err)
		return
	}
	ctx.Tell(ctx.Ref(), conn)
}
