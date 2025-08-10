package main

import (
	"fmt"

	"github.com/kercylan98/vivid/pkg/vivid"
)

func main() {
	var wait = make(chan struct{})
	system := vivid.NewActorSystem()
	system.SpawnOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch context.Message().(type) {
			case *vivid.OnLaunch:
				fmt.Println("hello")
				close(wait)
			}
		})
	})

	<-wait

	system.Shutdown(true)
}
