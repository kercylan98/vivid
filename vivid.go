package vivid

type (
	ActorPath = string
	Behavior  = func(ctx ActorContext)
)
