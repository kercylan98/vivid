package ves

import (
	"reflect"

	"github.com/kercylan98/vivid"
)

type ActorSpawnedEvent struct {
	ActorRef vivid.ActorRef
	Type     reflect.Type
}
