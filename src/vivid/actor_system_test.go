package vivid_test

import (
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
	"time"
)

func TestActorSystemImpl_ActorOf(t *testing.T) {
	system := vivid.NewActorSystem(vivid.NewActorSystemConfig()).StartP()
	system.ActorOf(vivid.ActorProviderFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			t.Log("Hello, World!")
		})
	}))

	time.Sleep(time.Second)
}

func BenchmarkActorSystemImpl_Tell(b *testing.B) {
	system := vivid.NewActorSystem(vivid.NewActorSystemConfig()).StartP()
	ref := system.ActorOf(vivid.ActorProviderFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {})
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Tell(ref, i)
	}
	b.StopTimer()
}
