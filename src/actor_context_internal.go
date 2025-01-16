package vivid

import (
	"log/slog"
	"sync/atomic"
)

const (
	actorStatusAlive       uint32 = iota // Actor 存活状态
	actorStatusRestarting                // Actor 正在重启
	actorStatusTerminating               // Actor 正在终止
	actorStatusTerminated                // Actor 已终止
)

var (
	_ Recipient = (*internalActorContext)(nil) // 确保 internalActorContext 实现了 Recipient 接口
	_ Process   = (*internalActorContext)(nil) // 确保 internalActorContext 实现了 Process 接口
)

type internalActorContext struct {
	*actorContext
	ref        ActorRef      // Actor 引用
	mailbox    Mailbox       // Actor 的邮箱
	terminated atomic.Bool   // Actor 是否已终止
	status     atomic.Uint32 // Actor 状态
	envelope   Envelope      // 当前消息
}

func (ctx *internalActorContext) init(actorContext *actorContext, mailbox Mailbox) {
	ctx.actorContext = actorContext
	ctx.mailbox = mailbox

	ctx.Logger().Debug("generated", slog.String("actor", ctx.ref.String()))

	ctx.Send(getEnvelopeBuilder().StandardOf(ctx.ref, ctx.ref, SystemMessage, onLaunch))
}

func (ctx *internalActorContext) GetID() ID {
	return ctx.ref
}

func (ctx *internalActorContext) Send(envelope Envelope) {
	switch envelope.GetMessage().(type) {
	case *onResumeMailboxMessage:
		ctx.mailbox.Resume()
	case *onSuspendMailboxMessage:
		ctx.mailbox.Suspend()
	default:
		ctx.mailbox.Delivery(envelope)
	}
}

func (ctx *internalActorContext) Terminated() bool {
	return ctx.terminated.Load()
}

func (ctx *internalActorContext) OnTerminate(operator ID) {
	ctx.terminated.Store(true)
}

func (ctx *internalActorContext) sendToProcess(envelope Envelope) {
	process, daemon := ctx.actorSystem.processManager.getProcess(envelope.GetReceiver())
	if daemon {
		ctx.Logger().Warn("sendToProcess", slog.Any("process not found", envelope))
		return
	}
	process.Send(envelope)
}

func (ctx *internalActorContext) onAccident(reason any) {
	//TODO implement me
	panic("implement me")
}

func (ctx *internalActorContext) OnReceiveEnvelope(envelope Envelope) {
	if ctx.status.Load() >= actorStatusTerminating {
		ctx.Logger().Warn("OnReceiveEnvelope", slog.String("actor is terminating", ctx.ref.GetPath()))
		ctx.sendToProcess(getEnvelopeBuilder().StandardOf(ctx.ref, ctx.actorSystem.Ref(), UserMessage, envelope))
		return
	}

	ctx.onProcessMessage(envelope)
}

func (ctx *internalActorContext) onProcessMessage(envelope Envelope) {
	ctx.envelope = envelope
	switch envelope.GetMessageType() {
	case SystemMessage:
		ctx.onProcessSystemMessage(envelope)
	case UserMessage:
		ctx.onProcessUserMessage(envelope)
	default:
		panic("unknown message type")
	}
}

func (ctx *internalActorContext) onProcessSystemMessage(envelope Envelope) {
	switch envelope.GetMessage().(type) {
	case *OnLaunch:
		ctx.onProcessMessage(getEnvelopeBuilder().ConvertTypeOf(envelope, UserMessage))
	default:
		panic("unknown system message")
	}
}

func (ctx *internalActorContext) onProcessUserMessage(envelope Envelope) {
	defer func() {
		// 系统消息事故需严格确保正常，必须抛出异常，所以仅在用户消息中处理
		if reason := recover(); reason != nil {
			ctx.onAccident(reason)
		}
	}()

	ctx.actor.OnReceive(ctx)
}
