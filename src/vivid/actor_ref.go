package vivid

import "github.com/kercylan98/wasteland/src/wasteland"

var (
	_ ActorRef            = (*actorRefImpl)(nil)
	_ actorRefProcessInfo = (*actorRefImpl)(nil)
)

type ActorRef interface {
	Address() Address

	Path() Path

	Equal(ref ActorRef) bool
}

type actorRefProcessInfo interface {
	ActorRef

	generateSub(path Path) ActorRef

	processId() wasteland.ProcessId
}
