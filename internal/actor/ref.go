package actor

import (
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/kercylan98/vivid"
)

var (
	_ vivid.ActorRef = (*Ref)(nil)
)

func NewRef(address, path string) *Ref {
	return &Ref{
		address: address,
		path:    path,
	}
}

type Ref struct {
	address string
	path    string
	cache   atomic.Pointer[vivid.Mailbox]
}

func (r *Ref) GetPath() string {
	return r.path
}

func (r *Ref) GetAddress() string {
	return r.address
}

func (r *Ref) Equals(other vivid.ActorRef) bool {
	if other == nil {
		return false
	}
	return r.GetAddress() == other.GetAddress() && r.GetPath() == other.GetPath()
}

func (r *Ref) Clone() vivid.ActorRef {
	return NewRef(r.GetAddress(), r.GetPath())
}

func (r *Ref) ToActorRefs() vivid.ActorRefs {
	return vivid.ActorRefs{r}
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
