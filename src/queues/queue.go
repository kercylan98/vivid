package queues

type Queue interface {
	Push(m any)
	Pop() any
}
