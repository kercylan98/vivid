package vividtemp

import (
	"github.com/kercylan98/wasteland/src/wasteland"
	"time"
)

var _ actorContextTransport = (*actorContextTransportImpl)(nil)

func newActorContextTransport(ctx ActorContext, process actorContextProcess) *actorContextTransportImpl {
	return &actorContextTransportImpl{
		ActorContext: ctx,
		process:      process,
	}
}

type actorContextTransport interface {
	ActorContext

	tell(target ActorRef, priority wasteland.MessagePriority, message Message)

	probe(target ActorRef, priority wasteland.MessagePriority, message Message)

	ask(target ActorRef, priority wasteland.MessagePriority, message Message, timeout ...time.Duration) Future

	reply(target ActorRef, message Message)
}

type actorContextTransportImpl struct {
	ActorContext
	process actorContextProcess
}

func (a *actorContextTransportImpl) tell(target ActorRef, priority wasteland.MessagePriority, message Message) {
	if a.ActorContext.Ref().Equal(target) {
		a.process.HandleMessage(nil, priority, message)
		return
	}

	registry := a.ActorContext.System().(actorSystemProcess).getProcessRegistry()
	process, err := registry.Get(target.(actorRefProcessInfo).processId())
	if err != nil {
		panic(err)
	}
	process.(wasteland.ProcessHandler).HandleMessage(nil, priority, message)
}

func (a *actorContextTransportImpl) probe(target ActorRef, priority wasteland.MessagePriority, message Message) {
	message = &addressableMessage{
		Sender:  a.ActorContext.Ref(),
		Message: message,
	}
	a.tell(target, priority, message)
}

func (a *actorContextTransportImpl) ask(target ActorRef, priority wasteland.MessagePriority, message Message, timeout ...time.Duration) Future {
	//TODO implement me
	panic("implement me")
}

func (a *actorContextTransportImpl) reply(target ActorRef, message Message) {
	//TODO implement me
	panic("implement me")
}
