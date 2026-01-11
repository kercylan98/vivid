package actor

import (
	"log/slog"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

func NewTestSystem(t *testing.T, options ...vivid.ActorSystemOption) *TestSystem {
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
	sys.System = newSystem(sys, options...)
	if err := sys.System.Start(); err != nil {
		t.Fatal(err)
	}
	return sys
}

type TestSystem struct {
	*System
	*testing.T
}
