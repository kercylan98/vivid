package vividtemp

import (
	"encoding/gob"
	"github.com/kercylan98/wasteland/src/wasteland"
	"strings"
)

func init() {
	gob.RegisterName("_actorRefImpl", &actorRefImpl{})
}

func newActorRef(pid wasteland.ProcessId) ActorRef {
	return &actorRefImpl{ProcessId: pid}
}

type actorRefImpl struct {
	wasteland.ProcessId
}

func (ref *actorRefImpl) Equal(other ActorRef) bool {
	return ref == other || (ref.Path() == other.Path() && ref.Address() == other.Address())
}

func (ref *actorRefImpl) generateSub(path Path) ActorRef {
	refPath := ref.Path()
	if refPath == "/" || refPath == "" {
		path = "/" + path
	} else {
		path = strings.TrimRight(ref.Path()+"/"+path, "/")
	}

	return &actorRefImpl{
		ProcessId: wasteland.NewProcessId(ref.ProcessId, path),
	}
}
