package cluster

import "github.com/kercylan98/vivid"

type ActorRefParser = func(address string, path string) (vivid.ActorRef, error)
