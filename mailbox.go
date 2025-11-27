package vivid

type Mailbox interface {
	Enqueue(envelop Envelop)
	Dequeue() Envelop
}

type Envelop interface {
	Sender() ActorRef
	Message() Message
}
