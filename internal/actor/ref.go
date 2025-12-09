package actor

import (
	"net"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/transparent"
)

var (
	_ vivid.ActorRef = &Ref{}
)

var zeroAddress = &net.UDPAddr{IP: net.IPv4zero, Port: 0}

func NewRef(address net.Addr, path vivid.ActorPath) *Ref {
	if address == nil {
		address = zeroAddress
	}
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

func newAgentRef(agent vivid.ActorRef) *agentRef {
	return &agentRef{
		ref:   NewRef(agent.GetAddress(), agent.GetPath()+"@future@"+uuid.NewString()),
		agent: agent,
	}
}

type agentRef struct {
	ref   vivid.ActorRef // 自身的 ActorRef
	agent vivid.ActorRef // 被代理的 ActorRef
}
