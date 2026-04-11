package remoting

import (
	"fmt"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/pkg/log"
)

type NetworkEnvelopHandler interface {
	HandleRemotingEnvelop(system bool, sender, receiver string, messageInstance any) error
	HandleFailedRemotingEnvelop(envelop vivid.Envelop)
}

type FailedEnvelopHandler interface {
	HandleFailedRemotingEnvelop(envelop vivid.Envelop)
}

type ActorSystemEnvelopTarget interface {
	Tell(recipient vivid.ActorRef, message vivid.Message)
	HeartbeatProbe(ref vivid.ActorRef) *vivid.Heartbeat
	ParseRef(actorRef string) (vivid.ActorRef, error)
	ResolveMailbox(receiver vivid.ActorRef) vivid.Mailbox
	Logger() log.Logger
}

func NewActorSystemEnvelopHandler(target ActorSystemEnvelopTarget, failedHandler FailedEnvelopHandler) NetworkEnvelopHandler {
	return &actorSystemEnvelopHandler{
		target:        target,
		failedHandler: failedHandler,
	}
}

type actorSystemEnvelopHandler struct {
	target        ActorSystemEnvelopTarget
	failedHandler FailedEnvelopHandler
}

func (h *actorSystemEnvelopHandler) HandleRemotingEnvelop(system bool, sender, receiver string, messageInstance any) error {
	if h.target == nil {
		return vivid.ErrorIllegalArgument.WithMessage("actor system envelop target is nil")
	}

	var (
		senderRef   vivid.ActorRef
		receiverRef vivid.ActorRef
		err         error
	)
	if sender != "" {
		senderRef, err = h.target.ParseRef(sender)
		if err != nil {
			h.target.Logger().Warn("invalid sender ref", log.String("ref", sender), log.Any("err", err))
			return fmt.Errorf("%w: invalid sender ref, %s", err, sender)
		}
	}
	receiverRef, err = h.target.ParseRef(receiver)
	if err != nil {
		h.target.Logger().Warn("invalid receiver ref", log.String("ref", receiver), log.Any("err", err))
		return fmt.Errorf("%w: invalid receiver ref, %s", err, receiver)
	}

	if heartbeat, ok := messageInstance.(*vivid.Heartbeat); ok && heartbeat.Ref == nil {
		h.target.Tell(senderRef, h.HeartbeatProbe(receiverRef))
		return nil
	}

	receiverMailbox := h.target.ResolveMailbox(receiverRef)
	receiverMailbox.Enqueue(mailbox.NewEnvelop(system, senderRef, receiverRef, messageInstance))
	return nil
}

func (h *actorSystemEnvelopHandler) HandleFailedRemotingEnvelop(envelop vivid.Envelop) {
	if h.failedHandler != nil {
		h.failedHandler.HandleFailedRemotingEnvelop(envelop)
	}
}

func (h *actorSystemEnvelopHandler) HeartbeatProbe(ref vivid.ActorRef) *vivid.Heartbeat {
	return h.target.HeartbeatProbe(ref)
}
