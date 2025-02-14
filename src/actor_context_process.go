package vivid

import (
	"github.com/kercylan98/go-log/log"
	"sync/atomic"
)

var (
	_ ActorContextProcess = (*actorContextProcess)(nil)
	_ Process             = (*actorContextProcess)(nil)
)

func newActorContextProcess(ctx ActorContext, ref ActorRef, mailbox Mailbox) *actorContextProcess {
	return &actorContextProcess{
		ActorContext: ctx,
		ref:          ref,
		mailbox:      mailbox,
	}
}

type actorContextProcess struct {
	ActorContext
	ref        ActorRef    // Actor 引用
	mailbox    Mailbox     // Actor 的邮箱
	terminated atomic.Bool // Actor 是否已终止
}

func (ctx *actorContextProcess) getProcessId() ActorRef {
	return ctx.GetID()
}

func (ctx *actorContextProcess) getProcess() Process {
	return ctx
}

func (ctx *actorContextProcess) GetID() ID {
	return ctx.ref
}

// Send 该函数为 Process 接口的实现，用于将消息交由邮箱处理
//   - 内部不建议甚至请拒绝直接调用该函数，除非你明确知道你在做什么
//   - 如果是对于自身的消息且无需考虑优先级的消息，可直接调用 actorContext.onProcessMessage 函数来得到更高效的处理
//   - 如果是对于其他 Actor 的消息，应该调用 actorContext.sendToProcess 函数来发送消息
func (ctx *actorContextProcess) Send(envelope Envelope) {
	switch envelope.GetMessage().(type) {
	case *onResumeMailboxMessage:
		ctx.mailbox.Resume()
	case *onSuspendMailboxMessage:
		ctx.mailbox.Suspend()
	default:
		ctx.mailbox.Delivery(envelope)
	}
}

func (ctx *actorContextProcess) Terminated() bool {
	return ctx.terminated.Load()
}

func (ctx *actorContextProcess) OnTerminate(operator ID) {
	ctx.terminated.Store(true)
}

func (ctx *actorContextProcess) sendToProcess(envelope Envelope) {
	process, daemon := ctx.System().getProcessManager().getProcess(envelope.GetReceiver())
	if daemon {
		ctx.Logger().Warn("sendToProcess", log.Any("onReceiveRemoteStreamMessage not found", envelope))
		return
	}
	process.Send(envelope)
}

func (ctx *actorContextProcess) sendToSelfProcess(envelope Envelope) {
	ctx.Send(envelope)
}
