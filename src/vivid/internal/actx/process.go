package actx

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/addressing"
	"github.com/kercylan98/wasteland/src/wasteland"
)

const (
	UserMessage   = wasteland.MessagePriority(0)
	SystemMessage = wasteland.MessagePriority(1)
)

var _ actor.ProcessContext = (*Process)(nil)

func NewProcess(ctx actor.Context) *Process {
	return &Process{
		ctx: ctx,
	}
}

type Process struct {
	ctx actor.Context
}

func (p *Process) Initialize() {

}

func (p *Process) Terminate(operator wasteland.ResourceLocator) {

}

func (p *Process) Terminated() bool {
	return p.ctx.LifecycleContext().Status() == lifecycleStatusTerminated
}

func (p *Process) GetID() wasteland.ResourceLocator {
	return p.ctx.MetadataContext().Ref()
}

func (p *Process) HandleMessage(sender wasteland.ResourceLocator, priority wasteland.MessagePriority, message wasteland.Message) {
	if sender != nil {
		message = addressing.NewMessage(sender.(actor.Ref), message)
	}

	if priority == UserMessage {
		p.ctx.MetadataContext().Config().Mailbox.HandleUserMessage(message)
	} else {
		p.ctx.MetadataContext().Config().Mailbox.HandleSystemMessage(message)
	}
}
