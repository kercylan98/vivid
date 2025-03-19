package vivid_test

import (
	"fmt"
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
)

func TestActorSystem_ActorOf(t *testing.T) {
	system := vivid.NewActorSystem().StartP()

	ref := system.ActorOf(vivid.ActorProviderFN(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			fmt.Println(1)
		})
	}))

	system.Tell(ref, 1)
}

func BenchmarkActorSystem_ActorOf(b *testing.B) {
	system := vivid.NewActorSystem().StartP()

	ref := system.ActorOf(vivid.ActorProviderFN(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {

		})
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Tell(ref, i)
	}
	b.StopTimer()
}
