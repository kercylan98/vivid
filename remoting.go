package vivid

import "net"

type RemotingActor interface {
	Actor

	GetBindAddress() net.Addr
	GetAdvertiseAddress() net.Addr
}

type RemotingConnActor interface {
	Actor

	GetAddress() net.Addr
}
