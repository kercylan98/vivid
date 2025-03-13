package vivid

type ActorRef interface {
	Address() Address

	Path() Path

	Equal(ref ActorRef) bool
}
