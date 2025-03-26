package vivid

type ActorRef interface {
    Address() Address

    Path() Path

    String() string

    URL() string
}
