package vivid_test

import (
	vivid "github.com/kercylan98/vivid/engine/v1"
	"testing"
	"time"
)

func TestActorSystemConfiguration_WithMetrics(t *testing.T) {
	system := NewTestActorSystem(t, vivid.ActorSystemConfiguratorFN(func(config *vivid.ActorSystemConfiguration) {
		config.WithMetrics(true)
	}))

	system.SpawnOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {

		})
	})

	time.Sleep(time.Second)
	system.Shutdown(true)
}
