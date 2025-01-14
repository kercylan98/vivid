package vivid

import "sync/atomic"

var (
	_ Recipient = (*internalActorContext)(nil) // 确保 internalActorContext 实现了 Recipient 接口
	_ Process   = (*internalActorContext)(nil) // 确保 internalActorContext 实现了 Process 接口
)

type internalActorContext struct {
	*actorContext
	ref        ActorRef
	mailbox    Mailbox
	terminated atomic.Bool
}

func (a *internalActorContext) init(ctx *actorContext, mailbox Mailbox) {
	a.actorContext = ctx
	a.mailbox = mailbox

	a.Send(getEnvelopeBuilder().StandardOf(a.ref, a.ref, SystemMessage, onLaunch))
}

func (a *internalActorContext) GetID() ID {
	return a.ref
}

func (a *internalActorContext) Send(envelope Envelope) {
	switch envelope.GetMessage().(type) {
	case *onResumeMailboxMessage:
		a.mailbox.Resume()
	case *onSuspendMailboxMessage:
		a.mailbox.Suspend()
	default:
		a.mailbox.Delivery(envelope)
	}
}

func (a *internalActorContext) Terminated() bool {
	return a.terminated.Load()
}

func (a *internalActorContext) OnTerminate(operator ID) {
	a.terminated.Store(true)
}

func (a *internalActorContext) OnReceiveEnvelope(envelope Envelope) {
	a.envelope = envelope
	a.actor.OnReceive(a)
}

func (a *internalActorContext) OnAccident(reason any) {
	//TODO implement me
	panic("implement me")
}
