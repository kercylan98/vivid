package remoting

import (
	"context"

	"github.com/kercylan98/vivid"
)

var _ vivid.Mailbox = (*Mailbox)(nil)

func NewMailbox(ctx context.Context, actorLiaison vivid.ActorLiaison, remotingRef vivid.ActorRef, failedHandler FailedEnvelopHandler, admission *OutboundAdmission) *Mailbox {
	return &Mailbox{
		ctx:           ctx,
		actorLiaison:  actorLiaison,
		remotingRef:   remotingRef,
		failedHandler: failedHandler,
		admission:     admission,
	}
}

// Mailbox 是系统共享的传输入口，目标由每条信封的 Receiver 决定，不保存目标位置。
type Mailbox struct {
	ctx           context.Context
	actorLiaison  vivid.ActorLiaison
	remotingRef   vivid.ActorRef
	failedHandler FailedEnvelopHandler
	admission     *OutboundAdmission
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
	pending, err := m.admission.reserve(envelop)
	if err != nil {
		m.admission.reject(envelop, err)
		return
	}
	m.actorLiaison.Tell(m.remotingRef, pending)
}

func (m *Mailbox) Pause() {}

func (m *Mailbox) Resume() {}

func (m *Mailbox) IsPaused() bool { return false }
