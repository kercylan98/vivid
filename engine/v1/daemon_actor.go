package vivid

func newDaemonActor(ctx *actorContext) *daemonActor {
    return &daemonActor{
        actorContext: ctx,
    }
}

type daemonActor struct {
    *actorContext
}

func (d *daemonActor) Receive(context ActorContext) {
    switch context.Message().(type) {
    case *OnLaunch:
        d.actorContext = context.(*actorContext)
    }
}
