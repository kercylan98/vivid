package gateway

import (
	"github.com/kercylan98/vivid/pkg/vivid"
)

var _ vivid.Actor = (*sessionActor)(nil)

func newSessionActor(transport Transport, codec Codec) *sessionActor {
	return &sessionActor{
		transport: transport,
		codec:     codec,
	}
}

type sessionActor struct {
	transport Transport
	sessionId *sessionId
	codec     Codec
}

func (s *sessionActor) Receive(context vivid.ActorContext) {
	switch m := context.Message().(type) {
	case *vivid.OnLaunch:
		s.onLaunch(context, m)
	case []byte:
		s.onBytes(context, m)
	}
}

func (s *sessionActor) onLaunch(context vivid.ActorContext, m *vivid.OnLaunch) {
	s.sessionId = newSessionId(context.Ref(), s.transport)
	_ = vivid.TypedAsk[*sessionId](context, context.Parent(), s.sessionId).Wait() // 内存级别不可能失败

	go s.startMessageLoop(context)
}

func (s *sessionActor) startMessageLoop(context vivid.ActorContext) {
	for {
		data, err := s.transport.Read()
		if err != nil {
			context.Tell(context.Ref(), err)
			return
		}
		context.Tell(context.Ref(), data)
	}
}

func (s *sessionActor) onBytes(context vivid.ActorContext, m []byte) {
	c2s, err := s.codec.Decode(m)
	if err != nil {
		context.Tell(context.Ref(), err)
		return
	}

	// TODO: 分发到对应服务
	_ = c2s
}
