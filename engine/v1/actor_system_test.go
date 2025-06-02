package vivid_test

import (
	vivid "github.com/kercylan98/vivid/engine/v1"
	"testing"
	"time"
)

func TestNewActorSystem(t *testing.T) {
	sys := vivid.NewActorSystem()
	sys.SpawnOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch context.Message().(type) {
			case *vivid.OnLaunch:
				context.Logger().Info("OnLaunch")
			}
		})
	})

	time.Sleep(100 * time.Millisecond)
}
