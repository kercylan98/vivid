package vivid

var _ Actor = (*guardActor)(nil)

type guardActor struct {
}

func (r *guardActor) OnReceive(ctx ActorContext) {

}
