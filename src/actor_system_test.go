package vivid_test

import (
	vivid "github.com/kercylan98/vivid/src"
	"testing"
	"time"
)

func TestActorSystem(t *testing.T) {
	sys := vivid.NewActorSystem().StartP()

	sys.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				t.Log("Actor launched")
			}
		})
	})

	time.Sleep(time.Second)
}
