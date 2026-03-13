package actor

import (
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/internal/utils"
)

var (
	_ vivid.ActorRef             = (*Ref)(nil)
	_ serialization.MessageCodec = (*Ref)(nil)
	_ serialization.MessageCodec = (*AgentRef)(nil)
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
	address     string
	path        string
	cache       atomic.Pointer[vivid.Mailbox]
	stringCache atomic.Pointer[string]
}

func (r *Ref) GetPath() string {
	return r.path
}

func (r *Ref) GetAddress() string {
	return r.address
}

func (r *Ref) Equals(other vivid.ActorRef) bool {
	ref, ok := other.(*Ref)
	return ok && r.GetPath() == ref.GetPath() && r.GetAddress() == ref.GetAddress()
}

func (r *Ref) Clone() vivid.ActorRef {
	return &Ref{
		address: r.address,
		path:    r.path,
	}
}

func (r *Ref) ToActorRefs() vivid.ActorRefs {
	return vivid.ActorRefs{r}
}

func (r *Ref) String() string {
	ptr := r.stringCache.Load()
	if ptr == nil {
		ptr = new(string)
		*ptr = utils.FormatRefString(r.GetAddress(), r.GetPath())
		r.stringCache.CompareAndSwap(nil, ptr)
	}
	return *ptr
}

func (r *Ref) IsVirtual() bool {
	return false
}

// Encode implements [serialization.MessageCodec].仅序列化 address、path，不包含 cache。
func (r *Ref) Encode(writer *serialization.Writer, message any) error {
	ref := message.(*Ref)
	return writer.Write(ref.GetAddress(), ref.GetPath()).Err()
}

// Decode implements [serialization.MessageCodec].
func (r *Ref) Decode(reader *serialization.Reader, message any) error {
	ref := message.(*Ref)
	return reader.Read(&ref.address, &ref.path)
}

func NewAgentRef(agent *Ref) (*AgentRef, error) {
	if agent == nil {
		return nil, vivid.ErrorRefNilAgent
	}
	// agent 一定合法的，且 agentFutureMaker 及 uuid 的组合一定不会返回 error。
	ref, _ := agent.Child(agentFutureMarker + uuid.NewString())
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

// Child 基于当前 Ref 快速生成子 Ref。
func (r *Ref) Child(path string) (*Ref, error) {
	if strings.TrimSpace(path) == "" {
		return nil, vivid.ErrorRefInvalidPath
	}
	return NewRef(r.address, utils.JoinPath(r.path, path))
}

// Encode implements [serialization.MessageCodec].仅序列化 ref、agent，不包含 cache。
func (a *AgentRef) Encode(writer *serialization.Writer, message any) error {
	agent := message.(*AgentRef)
	return writer.Write(agent.ref, agent.agent).Err()
}

// Decode implements [serialization.MessageCodec].
func (a *AgentRef) Decode(reader *serialization.Reader, message any) error {
	agent := message.(*AgentRef)
	return reader.Read(&agent.ref, &agent.agent)
}
