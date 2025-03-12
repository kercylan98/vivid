package vivid

var _ Actor = (*guide)(nil)

type guide struct {
}

func (g *guide) OnReceive(ctx ActorContext) {}
