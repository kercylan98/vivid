package main

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid"
)

func main() {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	consumer := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				ctx.Logger().Info("received", log.String("message", m))
			case int: // calc num * 2
				ctx.Reply(m * 2)
			}
		})
	})

	system.Tell(consumer, "Hello, world!")

	future := system.Ask(consumer, 10)
	result, err := future.Result()
	if err != nil {
		panic(err)
	}

	system.Logger().Info("result: ", log.Int("10*2", result.(int)))
}
