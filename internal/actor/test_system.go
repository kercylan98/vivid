package actor

import (
	"log/slog"
	"net"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

func NewTestSystem(t *testing.T, options ...vivid.ActorSystemOption) *TestSystem {
	return NewTestSystemWithBeforeStartHandler(t, nil, options...)
}

func NewTestSystemWithBeforeStartHandler(t *testing.T, beforeStartHandler func(system *TestSystem), options ...vivid.ActorSystemOption) *TestSystem {
	options = append([]vivid.ActorSystemOption{
		vivid.WithActorSystemLogger(log.NewSLogLogger(slog.New(log.NewTextHandler(t.Output(), &log.HandlerOptions{
			AddSource:   true,
			Level:       log.LevelDebug,
			ReplaceAttr: nil,
		})))),
	}, options...)
	sys := &TestSystem{
		T: t,
	}
	sys.System = newSystem(sys, beforeStartHandler, options...).Unwrap()
	return sys
}

type TestSystem struct {
	*System
	*testing.T
	remotingListenerBindEvents []func(listener net.Listener)
}

// RegisterRemotingListenerBindEvent 用于获取远程监听器。
func (s *TestSystem) RegisterRemotingListenerBindEvent(handler func(listener net.Listener)) {
	s.remotingListenerBindEvents = append(s.remotingListenerBindEvents, handler)
}

// onBindRemotingListener 用于在远程监听器绑定时通知测试系统。
func (s *TestSystem) onBindRemotingListener(listener net.Listener) {
	for _, handler := range s.remotingListenerBindEvents {
		handler(listener)
	}
}
