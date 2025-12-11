package vivid

type ActorRef interface {
	GetAddress() string
	GetPath() ActorPath
	Equals(other ActorRef) bool
	Clone() ActorRef
}
