package actor

import (
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/utils"
)

var (
	_ vivid.ActorRef = (*Ref)(nil)
)

const agentFutureMarker = "@future@"
const LocalAddress = "localhost"

func NewRef(address, path string) (*Ref, error) {
	address, ok := utils.NormalizeAddress(address)
	if !ok {
		if address == "" {
			address = "<empty>"
		}
		return nil, vivid.ErrorRefInvalidAddress.WithMessage(address)
	}
	path, ok = utils.NormalizePath(path)
	if !ok {
		return nil, vivid.ErrorRefInvalidPath.WithMessage(path)
	}
	return &Ref{
		address: address,
		path:    path,
	}, nil
}

// ParseRef 将字符串解析为 *Ref，支持 "domain/path" 与 "host:port:path"。
func ParseRef(value string) (*Ref, error) {
	if value == "" {
		return nil, vivid.ErrorRefEmpty
	}
	if split := strings.Index(value, ":/"); split > 0 && strings.Contains(value[:split], ":") {
		return NewRef(value[:split], value[split+1:])
	}
	slash := strings.IndexByte(value, '/')
	if slash <= 0 {
		return nil, vivid.ErrorRefFormat
	}
	return NewRef(value[:slash], value[slash:])
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
	ref, err := NewRef(r.GetAddress(), r.GetPath())
	if err != nil {
		return nil
	}
	return ref
}

func (r *Ref) ToActorRefs() vivid.ActorRefs {
	return vivid.ActorRefs{r}
}

func (r *Ref) String() string {
	return utils.FormatRefString(r.GetAddress(), r.GetPath())
}

func NewAgentRef(agent *Ref) (*AgentRef, error) {
	if agent == nil {
		return nil, vivid.ErrorRefNilAgent
	}
	ref, err := agent.Child(agentFutureMarker + uuid.NewString())
	if err != nil {
		return nil, err
	}
	return &AgentRef{
		ref:   ref,
		agent: agent,
	}, nil
}

type AgentRef struct {
	ref   *Ref // 自身的 ActorRef
	agent *Ref // 被代理的 ActorRef
}

func (a *AgentRef) Ref() *Ref {
	return a.ref
}

func (a *AgentRef) Agent() *Ref {
	return a.agent
}

// Child 基于当前 Ref 快速生成子 Ref。
func (r *Ref) Child(path string) (*Ref, error) {
	if strings.TrimSpace(path) == "" {
		return nil, vivid.ErrorRefInvalidPath
	}
	return NewRef(r.address, utils.JoinPath(r.path, path))
}
