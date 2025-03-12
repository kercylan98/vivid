package vivid

func newActorContext(system ActorSystem, ref, parentRef ActorRef, config *ActorConfiguration) ActorContext {
	actorContextBasic := newActorContextBasic(ref, parentRef)
	actorContextConfigurationProvider := newActorContextConfigurationProvider(config)
	actorContextChildren := newActorContextChildren()
	actorContextMailboxMessageHandler := newActorContextMailboxMessageHandler()
	actorContextProcess := newActorContextProcess(actorContextBasic)

	return &actorContextImpl{
		actorContextConfigurationProvider: actorContextConfigurationProvider,
		actorContextChildren:              actorContextChildren,
		actorContextBasic:                 actorContextBasic,
		actorContextMailboxMessageHandler: actorContextMailboxMessageHandler,
		actorContextProcess:               actorContextProcess,
	}
}

type actorContextImpl struct {
	actorContextBasic
	actorContextConfigurationProvider
	actorContextChildren
	actorContextMailboxMessageHandler
	actorContextProcess
}
