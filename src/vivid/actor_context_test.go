package vivid_test

import (
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
)

func TestActorContext_Watch(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	wait := make(chan struct{})

	system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				child := ctx.ActorOf(vivid.ActorProviderFN(func() vivid.Actor {
					return vivid.ActorFN(func(ctx vivid.ActorContext) {})
				}))

				ctx.Watch(child)
				ctx.PoisonKill(child)
			case *vivid.OnDead:
				wait <- struct{}{}
			}
		})
	})

	<-wait
}
