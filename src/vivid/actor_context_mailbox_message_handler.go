package vivid

func newActorContextMailboxMessageHandler(ctx ActorContext) actorContextMailboxMessageHandler {
	return &actorContextMailboxMessageHandlerImpl{
		ctx: ctx,
	}
}

type actorContextMailboxMessageHandler interface {
	MailboxHandler
}

type actorContextMailboxMessageHandlerImpl struct {
	ctx ActorContext
}

func (a *actorContextMailboxMessageHandlerImpl) HandleSystemMessage(message Message) {

}

func (a *actorContextMailboxMessageHandlerImpl) HandleUserMessage(message Message) {

}
