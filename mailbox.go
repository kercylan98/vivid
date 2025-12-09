package vivid

type Mailbox interface {
	Enqueue(envelop Envelop)
}

type Envelop interface {
	Agent() ActorRef
	Sender() ActorRef
	Message() Message
}

type EnvelopHandler interface {
	HandleEnvelop(envelop Envelop)
}
