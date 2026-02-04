package actor

import (
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

func NewTestSystem(t *testing.T, options ...vivid.ActorSystemOption) *TestSystem {
	options = append([]vivid.ActorSystemOption{
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
	}, options...)

	sys := &TestSystem{
		T:      t,
		System: NewSystem(options...),
	}

	if err := sys.System.Start(); err != nil {
		t.Fatal(err)
	}
	return sys
}

type TestSystem struct {
	*System
	*testing.T
}
