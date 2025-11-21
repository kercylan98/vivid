package actor

import (
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/guard"
)

var (
	_ vivid.ActorSystem = &System{}
)

func NewSystem() *System {
	system := &System{
		actorContexts: sync.Map{},
	}
	system.Context = NewContext(system, nil, guard.NewGuardActor())
	return system
}

type System struct {
	*Context               // ActorSystem 本身就表示了根 Actor
	actorContexts sync.Map // 用于加速访问的 ActorContext 缓存
}
