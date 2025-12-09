package vivid

type Mailbox interface {
	Enqueue(envelop Envelop)
}

type Envelop interface {
	Sender() ActorRef
	Message() Message
}

type EnvelopHandler interface {
	HandleEnvelop(envelop Envelop)
}
