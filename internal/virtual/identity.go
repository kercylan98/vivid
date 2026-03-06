package virtual

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
)

func init() {
	messages.RegisterInternalMessage[*Identity]("virtual.Identity", onIdentityReader, onIdentityWriter)
}

var (
	_ vivid.ActorRef = (*Identity)(nil)
)

func NewIdentity(kind string, name string) *Identity {
	return &Identity{
		kind: kind,
		name: name,
	}
}

type Identity struct {
	kind string
	name string
}

func onIdentityReader(message any, reader *messages.Reader, codec messages.Codec) error {
	identity := message.(*Identity)
	return reader.ReadInto(identity.kind, identity.name)
}

func onIdentityWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	identity := message.(*Identity)
	return writer.WriteFrom(identity.kind, identity.name)
}

func (i *Identity) Clone() vivid.ActorRef {
	return &Identity{
		kind: i.kind,
		name: i.name,
	}
}

func (i *Identity) Equals(other vivid.ActorRef) bool {
	identity, ok := other.(*Identity)
	return ok && identity.kind == i.kind && identity.name == i.name
}

func (i *Identity) GetAddress() string {
	return "virtual:" + i.kind
}

func (i *Identity) GetPath() vivid.ActorPath {
	return i.name
}

func (i *Identity) String() string {
	return "virtual:" + i.kind + "/" + i.name
}

func (i *Identity) ToActorRefs() vivid.ActorRefs {
	return vivid.ActorRefs{i}
}

func (i *Identity) IsVirtual() bool {
	return true
}
