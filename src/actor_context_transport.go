package vivid

import "time"

var (
	_ actorContextTransportInternal = (*actorContextTransportImpl)(nil)
)

func newActorContextTransportImpl(ctx *actorContext) *actorContextTransportImpl {
	return &actorContextTransportImpl{
		ActorContext: ctx,
	}
}

type actorContextTransportImpl struct {
	ActorContext
	envelope Envelope // 当前消息
}

func (ctx *actorContextTransportImpl) setEnvelope(envelope Envelope) {
	ctx.envelope = envelope
}

func (ctx *actorContextTransportImpl) getEnvelope() Envelope {
	return ctx.envelope
}

func (ctx *actorContextTransportImpl) Sender() ActorRef {
	if ctx.envelope == nil {
		return nil
	}
	return ctx.envelope.GetSender()
}

func (ctx *actorContextTransportImpl) Message() Message {
	if ctx.envelope == nil {
		return nil
	}
	return ctx.envelope.GetMessage()
}

func (ctx *actorContextTransportImpl) Reply(message Message) {
	var target = ctx.envelope.GetAgent()
	if target == nil {
		target = ctx.Sender()
	}
	ctx.tell(target, message, UserMessage)
}

func (ctx *actorContextTransportImpl) Ping(target ActorRef, timeout ...time.Duration) (pong Pong, err error) {
	return pong, ctx.ask(target, ctx.getSystemConfig().FetchRemoteMessageBuilder().BuildOnPing(), SystemMessage, timeout...).Adapter(FutureAdapter[Pong](func(p Pong, err error) error {
		p = pong
		return err
	}))
}
