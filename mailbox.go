package vivid

type Mailbox interface {
	Enqueue(envelop Envelop)
}

type Envelop interface {
	System() bool
	Agent() ActorRef
	Sender() ActorRef
	Message() Message
}

type EnvelopHandler interface {
	HandleEnvelop(envelop Envelop)
}
