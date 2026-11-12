package remoting

import (
	"context"

	"github.com/kercylan98/vivid"
)

var _ vivid.Mailbox = (*Mailbox)(nil)

func NewMailbox(ctx context.Context, actorLiaison vivid.ActorLiaison, remotingRef vivid.ActorRef, failedHandler FailedEnvelopHandler) *Mailbox {
	return &Mailbox{
		ctx:           ctx,
		actorLiaison:  actorLiaison,
		remotingRef:   remotingRef,
		failedHandler: failedHandler,
	}
}

type Mailbox struct {
	ctx           context.Context
	actorLiaison  vivid.ActorLiaison
	remotingRef   vivid.ActorRef
	failedHandler FailedEnvelopHandler
}

func (m *Mailbox) Enqueue(envelop vivid.Envelop) {
	if m == nil || envelop == nil {
		return
	}
	if m.ctx != nil && m.ctx.Err() != nil {
		if m.failedHandler != nil {
			m.failedHandler.HandleFailedRemotingEnvelop(envelop)
		}
		return
	}
	if m.remotingRef == nil || m.actorLiaison == nil {
		if m.failedHandler != nil {
			m.failedHandler.HandleFailedRemotingEnvelop(envelop)
		}
		return
	}
	m.actorLiaison.Tell(m.remotingRef, envelop)
}

func (m *Mailbox) Pause() {}

func (m *Mailbox) Resume() {}

func (m *Mailbox) IsPaused() bool { return false }
