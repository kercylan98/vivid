package vivid_test

import (
	"github.com/kercylan98/go-log/log"
	vivid "github.com/kercylan98/vivid/src"
	"testing"
	"time"
)

func TestActorSystem(t *testing.T) {
	sys := vivid.NewActorSystem().StartP()

	ref := sys.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case vivid.OnLaunch:
				ctx.Logger().Info("OnLaunch")
			case int:
				ctx.Logger().Info("int")
			case string:
				ctx.Logger().Info("string")
				ctx.Reply("reply")
			}
		})
	})

	sys.Tell(ref, 1)

	v, err := sys.Ask(ref, "").Result()
	if err != nil {
		t.Fatal(err)
	}
	sys.Logger().Info("reply:", log.Any("message", v))
	time.Sleep(time.Second)
}
