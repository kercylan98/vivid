package actor

import (
	"net"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/internal/transparent"
	"github.com/kercylan98/vivid/internal/utils"
)

var (
	_ vivid.ActorRef = (*Ref)(nil)
)
var zeroAddress = &net.UDPAddr{IP: net.IPv4zero, Port: 0}

func init() {
	messages.SetActorRefFactory(func(address net.Addr, path string) any {
		return NewRef(address, path)
	})

	messages.RegisterInternalMessage(messages.RefMessageType,
		func() any { return &Ref{} },
		func(actorRefFactory messages.ActorRefFactory, message any, reader *messages.Reader) (err error) {
			r := message.(*Ref)
			var network, address string
			if err = reader.ReadInto(&network, &address, &r.path); err != nil {
				return err
			}
			r.address, err = utils.ResolveNetAddr(network, address)
			return err
		},
		func(actorRefFactory messages.ActorRefFactory, message any, writer *messages.Writer) error {
			r := message.(*Ref)
			network, address := r.address.Network(), r.address.String()
			return writer.WriteFrom(network, address, r.path)
		})
}

func NewRef(address net.Addr, path string) *Ref {
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
	path    string
	cache   atomic.Pointer[transparent.TransportContext]
}

func (r *Ref) GetPath() string {
	return r.path
}

func (r *Ref) GetAddress() net.Addr {
	return r.address
}

func (r *Ref) Equals(other vivid.ActorRef) bool {
	return r.GetAddress().Network() == other.GetAddress().Network() && r.GetAddress().String() == other.GetAddress().String() && r.GetPath() == other.GetPath()
}

func NewAgentRef(agent *Ref) *AgentRef {
	return &AgentRef{
		ref:   NewRef(agent.GetAddress(), agent.GetPath()+"@future@"+uuid.NewString()),
		agent: agent,
	}
}

type AgentRef struct {
	ref   *Ref // 自身的 ActorRef
	agent *Ref // 被代理的 ActorRef
}
