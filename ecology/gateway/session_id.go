package gateway

import "github.com/kercylan98/vivid/pkg/vivid"

var _ SessionId = (*sessionId)(nil)

type SessionId interface {
	GetId() string
	GetName() string
}

func newSessionId(ref vivid.ActorRef, transport Transport) *sessionId {
	return &sessionId{
		ActorRef: ref,
		id:       transport.GetSessionId(),
		name:     transport.GetSessionName(),
	}
}

type sessionId struct {
	vivid.ActorRef
	id   string
	name string
}

func (s *sessionId) GetId() string {
	return s.id
}

func (s *sessionId) GetName() string {
	return s.name
}
