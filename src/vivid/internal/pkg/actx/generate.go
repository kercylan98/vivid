package actx

import "github.com/kercylan98/vivid/src/vivid/internal/core/actor"

var _ actor.GenerateContext = (*Generate)(nil)

type Generate struct {
}

func (g *Generate) GenerateActorContext() actor.Context {

}
