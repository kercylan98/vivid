package vivid_test

import (
	vivid "github.com/kercylan98/vivid/src"
	"testing"
	"time"
)

func TestDefaultActorConfig_WithSlowMessageThreshold(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case vivid.OnLaunch:
				time.Sleep(time.Second)
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithSlowMessageThreshold(time.Millisecond * 900)
	})
}
