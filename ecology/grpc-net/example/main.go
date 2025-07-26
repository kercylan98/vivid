package main

import (
	"fmt"
	"github.com/kercylan98/vivid/grpc-net/example/messages"
	"github.com/kercylan98/vivid/grpc-net/grpcnet"
	"github.com/kercylan98/vivid/pkg/vivid"
)

func main() {
	system1 := vivid.NewActorSystemWithConfigurators(vivid.ActorSystemConfiguratorFN(func(c *vivid.ActorSystemConfiguration) {
		c.WithNetwork(grpcnet.NewNetworkConfiguration("127.0.0.1:19858"))
	}))
	system2 := vivid.NewActorSystemWithConfigurators(vivid.ActorSystemConfiguratorFN(func(c *vivid.ActorSystemConfiguration) {
		c.WithNetwork(grpcnet.NewNetworkConfiguration("127.0.0.1:19859"))
	}))

	ref := system1.SpawnOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch m := context.Message().(type) {
			case *messages.Message:
				fmt.Println(context.Sender(), m.Text)
				context.Reply(m)
			}
		})
	})
	ref = ref.Clone() // 抹去本地缓存

	system2.Tell(ref, &messages.Message{Text: "Hello, Vivid!"})
	echo, err := vivid.TypedAsk[*messages.Message](system2, ref, &messages.Message{Text: "Go go go"}).Result()
	if err != nil {
		panic(err)
	}
	fmt.Println(echo.Text)
}
