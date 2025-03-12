package vivid

func newActorContextMailboxMessageHandler(ctx ActorContext, basic actorContextBasic) actorContextMailboxMessageHandler {
	return &actorContextMailboxMessageHandlerImpl{
		ctx:   ctx,
		basic: basic,
	}
}

type actorContextMailboxMessageHandler interface {
	MailboxHandler
}

type actorContextMailboxMessageHandlerImpl struct {
	ctx   ActorContext
	basic actorContextBasic
}

func (a *actorContextMailboxMessageHandlerImpl) unwrapMessage(m Message) (sender ActorRef, message Message) {
	if am, cast := m.(*addressableMessage); cast {
		return am.Sender, am.Message
	}
	return nil, m
}

func (a *actorContextMailboxMessageHandlerImpl) HandleSystemMessage(message Message) {
	switch message.(type) {
	case *OnLaunch:
		a.basic.getActor().OnReceive(a.ctx)
	}
}

func (a *actorContextMailboxMessageHandlerImpl) HandleUserMessage(message Message) {

}
