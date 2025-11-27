package actor

import (
	"net"
	"sync/atomic"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/transparent"
)

var (
	_ vivid.ActorRef = &Ref{}
)

func NewRef(address net.Addr, path vivid.ActorPath) *Ref {
	return &Ref{
		address: address,
		path:    path,
	}
}

type Ref struct {
	address net.Addr
	path    vivid.ActorPath
	cache   atomic.Pointer[transparent.TransportContext]
}

func (r *Ref) GetPath() vivid.ActorPath {
	return r.path
}

func (r *Ref) GetAddress() net.Addr {
	return r.address
}
