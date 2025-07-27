package vivid_test

import (
	"fmt"
	"github.com/kercylan98/vivid/pkg/vivid"
)

func ExampleNewActorSystem() {
	system := vivid.NewActorSystem()
	defer func() {
		if err := system.Shutdown(true); err != nil {
			panic(err)
		}
	}()

	ref := system.SpawnOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			fmt.Println(ctx.Message())
		})
	})

	system.Tell(ref, "Hello Vivid!")
}
