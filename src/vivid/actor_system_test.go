package vivid_test

import (
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
	"time"
)

func TestActorSystem_ActorOf(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	system.ActorOf(vivid.ActorProviderFN(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				t.Log(ctx.Ref().Path(), ctx.Sender(), "Launch")
			}
		})
	}))

	time.Sleep(time.Second) // TODO: Shutdown 暂未实现，暂时用 Sleep 代替
}
