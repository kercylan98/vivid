package actx

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/addressing"
)

var _ actor.MessageContext = (*Message)(nil)

func NewMessage(ctx actor.Context) *Message {
	return &Message{
		ctx: ctx,
	}
}

type Message struct {
	ctx     actor.Context
	sender  actor.Ref    // 消息发送者（可能为 nil）
	message core.Message // 当前正在处理的消息
}

func (m *Message) Message() core.Message {
	return m.message
}

func (m *Message) Sender() actor.Ref {
	return m.sender
}

func (m *Message) parseRawMessage(message core.Message) core.Message {
	addressingMessage, ok := message.(*addressing.Message)
	if !ok {
		m.message = message
	} else {
		m.sender = addressingMessage.Sender
		m.message = addressingMessage.Message
	}
	return m.message
}

func (m *Message) HandleSystemMessage(message core.Message) {
	message = m.parseRawMessage(message)

	switch msg := message.(type) {
	case *actor.OnLaunch:
		m.HandleUserMessage(message)
	case *actor.OnKill:
		m.ctx.LifecycleContext().Kill(msg)
	case *actor.OnKilled:
		m.ctx.RelationContext().UnbindChild(m.sender)
	}
}

func (m *Message) HandleUserMessage(message core.Message) {
	message = m.parseRawMessage(message)

	switch msg := message.(type) {
	case *actor.OnLaunch:
		m.ctx.GenerateContext().Actor().OnReceive(m.ctx)
	case *actor.OnKill:
		m.ctx.LifecycleContext().Kill(msg)
	default:
		m.ctx.GenerateContext().Actor().OnReceive(m.ctx)
	}
}
