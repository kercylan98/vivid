package vivid

import (
	"encoding/gob"
	"github.com/kercylan98/wasteland/src/wasteland"
	"strings"
)

func init() {
	gob.RegisterName("_actorRefImpl", &actorRefImpl{})
}

func newActorRef(pid wasteland.ProcessId) ActorRef {
	return &actorRefImpl{pid: pid}
}

type actorRefImpl struct {
	pid wasteland.ProcessId
}

func (ref *actorRefImpl) Equal(other ActorRef) bool {
	return ref == other || (ref.Path() == other.Path() && ref.Address() == other.Address())
}

func (ref *actorRefImpl) processId() wasteland.ProcessId {
	return ref.pid
}

func (ref *actorRefImpl) Address() Address {
	return ref.pid.Address()
}

func (ref *actorRefImpl) generateSub(path Path) ActorRef {
	refPath := ref.Path()
	if refPath == "/" || refPath == "" {
		path = "/" + path
	} else {
		path = strings.TrimRight(ref.Path()+"/"+path, "/")
	}

	return &actorRefImpl{
		pid: wasteland.NewProcessId(ref.pid.(wasteland.Meta), path),
	}
}

func (ref *actorRefImpl) Path() Path {
	return ref.pid.Path()
}
