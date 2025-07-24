package vivid

func newDaemonActor() *daemonActor {
	return &daemonActor{}
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
