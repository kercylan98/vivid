package vivid_test

import (
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
)

func BenchmarkActorSystem_Tell(b *testing.B) {
	system := vivid.NewActorSystem().StartP()

	ref := system.ActorOf(vivid.ActorProviderFN(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {})
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Tell(ref, i)
	}
	b.StopTimer()
}
