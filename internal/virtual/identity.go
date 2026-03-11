package virtual

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
)

var (
	_ vivid.ActorRef             = (*Identity)(nil)
	_ serialization.MessageCodec = (*Identity)(nil)
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

// Decode implements [serialization.MessageCodec].
func (i *Identity) Decode(reader *serialization.Reader, message any) error {
	identity := message.(*Identity)
	return reader.Read(identity.kind, identity.name)
}

// Encode implements [serialization.MessageCodec].
func (i *Identity) Encode(writer *serialization.Writer, message any) error {
	identity := message.(*Identity)
	return writer.Write(identity.kind, identity.name).Err()
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
