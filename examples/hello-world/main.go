package main

import (
	"github.com/kercylan98/vivid/src/vivid"
)

func main() {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Logger().Info("Hello, World!")
			}
		})
	})
}
