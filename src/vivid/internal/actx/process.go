package actx

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
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

func (p *Process) Terminate(operator wasteland.ProcessId) {

}

func (p *Process) Terminated() bool {
	return false
}

func (p *Process) GetID() wasteland.ProcessId {
	return p.ctx.MetadataContext().Ref()
}

func (p *Process) HandleMessage(sender wasteland.ProcessId, priority wasteland.MessagePriority, message wasteland.Message) {
	if priority == UserMessage {
		p.ctx.MetadataContext().Config().Mailbox.HandleUserMessage(message)
	} else {
		p.ctx.MetadataContext().Config().Mailbox.HandleSystemMessage(message)
	}
}
