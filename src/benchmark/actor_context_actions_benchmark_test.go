package benchmark

import (
	vivid "github.com/kercylan98/vivid/src"
	"testing"
)

func BenchmarkActorContextTell(b *testing.B) {
	system := vivid.NewActorSystem().StartP()

	ref := system.ActorOf(vivid.ActorProviderFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {})
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Tell(ref, i)
	}
	b.StopTimer()

	//system.ShutdownP()
}

func BenchmarkActorContextAsk(b *testing.B) {
	system := vivid.NewActorSystem().StartP()

	ref := system.ActorOf(vivid.ActorProviderFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch v := ctx.Message().(type) {
			case int:
				ctx.Reply(v)
			}
		})
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Ask(ref, i).AssertWait()
	}
	b.StopTimer()

	system.ShutdownP()
}
