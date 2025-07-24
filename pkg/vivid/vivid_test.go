package vivid_test

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/pkg/vivid"
	"sync"
	"testing"
)

type TestActorSystem struct {
	vivid.ActorSystem
	t  *testing.T
	wg sync.WaitGroup
}

func NewTestActorSystem(t *testing.T, configurators ...vivid.ActorSystemConfigurator) *TestActorSystem {
	var defaultConfigurator = vivid.ActorSystemConfiguratorFN(func(c *vivid.ActorSystemConfiguration) {
		c.WithLogger(log.GetBuilder().FromConfigurators(log.LoggerConfiguratorFn(func(config log.LoggerConfiguration) {
			config.
				WithLeveler(log.LevelDebug).
				WithEnableColor(true).
				WithErrTrackLevel(log.LevelError).
				WithTrackBeautify(true).
				WithMessageFormatter(func(message string) string {
					return message
				})
		})))
	})
	configurators = append([]vivid.ActorSystemConfigurator{defaultConfigurator}, configurators...)

	return &TestActorSystem{
		t:           t,
		ActorSystem: vivid.NewActorSystemWithConfigurators(configurators...),
	}
}

func (t *TestActorSystem) WaitAdd(n int) *TestActorSystem {
	t.wg.Add(n)
	return t
}

func (t *TestActorSystem) WaitDone() *TestActorSystem {
	t.wg.Done()
	return t
}

func (t *TestActorSystem) Wait() {
	t.wg.Wait()
}

func (t *TestActorSystem) WaitFN(f func(system *TestActorSystem)) *TestActorSystem {
	f(t)
	t.Wait()
	return t
}

func (t *TestActorSystem) FN(f func(system *TestActorSystem)) *TestActorSystem {
	f(t)
	return t
}

func (t *TestActorSystem) AssertError(err error) {
	if err != nil {
		t.t.Error(err)
	}
}

func (t *TestActorSystem) AssertNil(value interface{}) {
	if value != nil {
		t.t.Errorf("expected nil, got %v", value)
	}
}

func (t *TestActorSystem) AssertNotNil(value interface{}) {
	if value == nil {
		t.t.Error("expected not nil")
	}
}

func (t *TestActorSystem) AssertEqual(n any, name string) {
	if n != name {
		t.t.Errorf("expected %v, got %v", name, n)
	}
}

func (t *TestActorSystem) Shutdown(poison bool, reason ...string) {
	t.AssertError(t.ActorSystem.Shutdown(poison, reason...))
}
