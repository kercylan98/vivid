package vivid

import "github.com/kercylan98/vivid/engine/v1/internal/processor"

type ActorRef = processor.UnitIdentifier

func NewActorRef(address, path string) ActorRef {
	return processor.NewCacheUnitIdentifier(address, path)
}
