package ref

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/wasteland/src/wasteland"
	"strings"
)

var (
	_ actor.Ref = &actorRef{}
)

func init() {
	gob.RegisterName("vivid:actorRef", &actorRef{})
}

// NewActorRef 创建根据指定的 wasteland.ProcessId 一个 ActorRef
func NewActorRef(pid wasteland.ProcessId) actor.Ref {
	return &actorRef{
		ProcessIdCache: pid.(wasteland.ProcessIdCache),
	}
}

type actorRef struct {
	wasteland.ProcessIdCache
}

func (ref *actorRef) Equal(other actor.Ref) bool {
	return ref == other || (ref.Path() == other.Path() && ref.Address() == other.Address())
}

func (ref *actorRef) GenerateSub(path core.Path) actor.Ref {
	refPath := ref.Path()
	if refPath == "/" || refPath == "" {
		path = "/" + path
	} else {
		path = strings.TrimRight(ref.Path()+"/"+path, "/")
	}

	return &actorRef{
		ProcessIdCache: wasteland.NewProcessId(ref.ProcessIdCache, path).(wasteland.ProcessIdCache),
	}
}
