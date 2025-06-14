package vivid_test

import (
	"github.com/kercylan98/vivid/core/vivid"
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
