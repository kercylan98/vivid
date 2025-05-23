package actx

import (
	"github.com/kercylan98/chrono/timing"
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

func (m *Message) HandleWith(message core.Message) {
	backup := m.message
	defer func() {
		m.message = backup
	}()
	m.message = message
	m.ctx.GenerateContext().Handle()
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
		m.ctx.LifecycleContext().TerminateTest(msg)
	case *actor.OnWatch:
		m.ctx.RelationContext().AddWatcher(m.sender)
	case *actor.OnUnwatch:
		m.ctx.RelationContext().RemoveWatcher(m.sender)
	case *actor.AccidentSnapshotEnd:
		m.ctx.LifecycleContext().AccidentEnd(msg.Snapshot)
	case actor.AccidentSnapshot:
		m.ctx.LifecycleContext().HandleAccidentSnapshot(msg)
	case *actor.OnPing:
		m.ctx.TransportContext().HandlePing(msg)
	case *actor.OnPong:
		m.ctx.TransportContext().HandlePong(msg, m.sender)
	}
}

func (m *Message) HandleUserMessage(message core.Message) {
	message = m.parseRawMessage(message)

	switch msg := message.(type) {
	case *actor.OnLaunch:
		m.ctx.GenerateContext().Handle()
	case *actor.OnKill:
		m.ctx.LifecycleContext().Kill(msg) // from poison
	case timing.Task:
		msg.Execute()
	default:
		m.ctx.GenerateContext().Handle()
	}
}
