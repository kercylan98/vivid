package vividtemp

import "github.com/kercylan98/wasteland/src/wasteland"

const (
	messagePriorityUser wasteland.MessagePriority = iota
	messagePrioritySystem
)

func newActorContextProcess(ctx ActorContext, basic actorContextBasic, config actorContextConfigurationProvider) actorContextProcess {
	return &actorContextProcessImpl{
		ctx:    ctx,
		base:   basic,
		config: config,
	}
}

type actorContextProcess interface {
	wasteland.Process
	wasteland.ProcessLifecycle
	wasteland.ProcessHandler
}

type actorContextProcessImpl struct {
	ctx    ActorContext
	base   actorContextBasic
	config actorContextConfigurationProvider
}

func (a *actorContextProcessImpl) GetID() wasteland.ProcessId {
	return a.base.getRef()
}

func (a *actorContextProcessImpl) Initialize() {

}

func (a *actorContextProcessImpl) HandleMessage(sender wasteland.ProcessId, priority wasteland.MessagePriority, message wasteland.Message) {
	mailbox := a.config.getConfig().Mailbox
	if sender != nil {
		message = &addressableMessage{
			Sender:  newActorRef(sender),
			Message: message,
		}
	}
	if priority == messagePriorityUser {
		mailbox.HandleUserMessage(message)
	} else {
		mailbox.HandleSystemMessage(message)
	}
}

func (a *actorContextProcessImpl) Terminate(operator wasteland.ProcessId) {
	//TODO implement me
	panic("implement me")
}

func (a *actorContextProcessImpl) Terminated() bool {
	return false
}
