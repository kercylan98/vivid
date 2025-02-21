package vivid

import (
	"fmt"
	"github.com/kercylan98/go-log/log"
)

type contextFunc func(ctx ActorContext)

var (
	_ Recipient = (*actorContextRecipient)(nil)
)

func newActorContextRecipient(ctx ActorContext) *actorContextRecipient {
	return &actorContextRecipient{
		ActorContext: ctx,
	}
}

type actorContextRecipient struct {
	ActorContext
}

func (ctx *actorContextRecipient) OnReceiveEnvelope(envelope Envelope) {
	if ctx.terminated() {
		switch envelope.GetMessage().(type) {
		case OnWatch, OnWatchStopped, contextFunc, *accidentFinished:
			// 此类消息在关闭后依旧可能被发送，需要经过处理以达到状态一致，处理中需要确保考虑到 Actor 不同状态下的处理逻辑
		case OnKill, OnUnwatch:
			// 此类消息在关闭后依旧可能被发送，不处理的效果等同于已经处理
			return
		default:
			ctx.Logger().Warn("OnReceiveEnvelope", log.String("actor is terminated", ctx.Ref().String()), log.String("sender", envelope.GetSender().String()), log.String("message", fmt.Sprintf("%T", envelope.GetMessage())))

			// 如果该 Actor 不是顶级 Actor，那么将消息传递给顶级 Actor 确保异常被记录
			// 如果已经是顶级 Actor，则说明 ActorSystem 正在关闭，需要丢弃消息
			if parent := ctx.Parent(); parent != nil {
				ctx.Tell(ctx.System().Ref(), envelope)
			}
			return
		}
	}

	ctx.onProcessMessage(ctx.persistentMessageParse(envelope))
}
