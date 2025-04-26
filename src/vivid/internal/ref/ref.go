package ref

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/wasteland/src/wasteland"
	"strings"
)

var (
	_ actor.Ref               = (*actorRef)(nil)
	_ wasteland.ResourceCache = (*actorRef)(nil)
)

// NewActorRef 创建根据指定的 wasteland.ProcessId 一个 ActorRef
func NewActorRef(resourceLocator wasteland.ResourceLocator) actor.Ref {
	return &actorRef{
		ResourceLocator: resourceLocator,
	}
}

type actorRef struct {
	wasteland.ResourceLocator
}

func (ref *actorRef) Load() wasteland.Process {
	if cache, ok := ref.ResourceLocator.(wasteland.ResourceCache); ok {
		return cache.Load()
	}
	return nil
}

func (ref *actorRef) Store(process wasteland.Process) {
	if cache, ok := ref.ResourceLocator.(wasteland.ResourceCache); ok {
		cache.Store(process)
	}
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
		ResourceLocator: wasteland.NewResourceLocator(ref.Address(), path),
	}
}

func (ref *actorRef) String() string {
	return ref.Path()
}

func (ref *actorRef) URL() string {
	return ref.Address() + ref.Path()
}
