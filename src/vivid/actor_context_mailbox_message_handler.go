package vivid

func newActorContextMailboxMessageHandler() actorContextMailboxMessageHandler {
	return &actorContextMailboxMessageHandlerImpl{}
}

type actorContextMailboxMessageHandler interface {
	MailboxHandler
}

type actorContextMailboxMessageHandlerImpl struct {
}

func (a *actorContextMailboxMessageHandlerImpl) HandleSystemMessage(message Message) {

}

func (a *actorContextMailboxMessageHandlerImpl) HandleUserMessage(message Message) {

}
