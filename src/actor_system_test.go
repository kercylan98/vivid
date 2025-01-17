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
				ctx.Reply("reply: " + ctx.Sender().String())
			}
		})
	})

	sys.Tell(ref, 1)

	if err := sys.Ask(ref, "").Adapter(
		vivid.FutureAdapter[string](func(s string, err error) error {
			sys.Logger().Info("reply:", log.Any("message", s))
			return nil
		})); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
}
