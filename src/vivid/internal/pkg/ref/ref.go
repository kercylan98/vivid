package ref

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/src/vivid/internal/features"
	"github.com/kercylan98/wasteland/src/wasteland"
	"strings"
)

var (
	_ core.ActorRef = &actorRef{}
)

func init() {
	gob.RegisterName("vivid:actorRef", &actorRef{})
}

// NewActorRef 创建根据指定的 wasteland.ProcessId 一个 ActorRef
func NewActorRef(pid wasteland.ProcessId) core.ActorRef {
	return &actorRef{
		ProcessId: pid,
	}
}

type actorRef struct {
	wasteland.ProcessId
}

func (ref *actorRef) Equal(other core.ActorRef) bool {
	return ref == other || (ref.Path() == other.Path() && ref.Address() == other.Address())
}

func (ref *actorRef) GenerateSub(path core.Path) core.ActorRef {
	refPath := ref.Path()
	if refPath == "/" || refPath == "" {
		path = "/" + path
	} else {
		path = strings.TrimRight(ref.Path()+"/"+path, "/")
	}

	return &actorRef{
		ProcessId: wasteland.NewProcessId(ref.ProcessId, path),
	}
}
