package actx

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

var _ actor.MessageContext = (*Message)(nil)

func NewMessage(ctx actor.Context) actor.MessageContext {
	return &Message{
		ctx: ctx,
	}
}

type Message struct {
	ctx actor.Context
}

func (m *Message) HandleSystemMessage(message core.Message) {
	m.ctx.GenerateContext().Actor().OnReceive(m.ctx)
}

func (m *Message) HandleUserMessage(message core.Message) {
	m.ctx.GenerateContext().Actor().OnReceive(m.ctx)
}
