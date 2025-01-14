package vivid

var _ Actor = (*rootActor)(nil)

type rootActor struct {
}

func (r *rootActor) OnReceive(ctx ActorContext) {
	
}
