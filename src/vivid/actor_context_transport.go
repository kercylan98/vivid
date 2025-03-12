package vivid

import (
	"github.com/kercylan98/wasteland/src/wasteland"
	"time"
)

var _ actorContextTransport = (*actorContextTransportImpl)(nil)

func newActorContextTransport(ctx ActorContext) *actorContextTransportImpl {
	return &actorContextTransportImpl{
		ctx: ctx,
	}
}

type actorContextTransport interface {
	tell(target ActorRef, priority wasteland.MessagePriority, message Message)

	probe(target ActorRef, priority wasteland.MessagePriority, message Message)

	ask(target ActorRef, priority wasteland.MessagePriority, message Message, timeout ...time.Duration) Future

	reply(target ActorRef, message Message)
}

type actorContextTransportImpl struct {
	ctx ActorContext
}

func (a *actorContextTransportImpl) tell(target ActorRef, priority wasteland.MessagePriority, message Message) {
	registry := a.ctx.System().(actorSystemProcess).getProcessRegistry()
	process, err := registry.Get(target.(actorRefProcessInfo).processId())
	if err != nil {
		panic(err)
	}
	process.(wasteland.ProcessHandler).HandleMessage(a.ctx.Ref().(actorRefProcessInfo).processId(), priority, message)
}

func (a *actorContextTransportImpl) probe(target ActorRef, priority wasteland.MessagePriority, message Message) {
	message = &addressableMessage{
		Sender:  a.ctx.Ref(),
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
