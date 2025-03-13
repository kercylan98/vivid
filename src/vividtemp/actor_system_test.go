package vividtemp_test

import (
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
	"time"
)

func TestActorSystemImpl_ActorOf(t *testing.T) {
	system := vividtemp.NewActorSystem(vividtemp.NewActorSystemConfig()).StartP()
	system.ActorOf(vividtemp.ActorProviderFn(func() vividtemp.Actor {
		return vividtemp.ActorFn(func(ctx vividtemp.ActorContext) {
			t.Log("Hello, World!")
		})
	}))

	time.Sleep(time.Second)
}

func BenchmarkActorSystemImpl_Tell(b *testing.B) {
	system := vividtemp.NewActorSystem(vividtemp.NewActorSystemConfig()).StartP()
	ref := system.ActorOf(vividtemp.ActorProviderFn(func() vividtemp.Actor {
		return vividtemp.ActorFn(func(ctx vividtemp.ActorContext) {})
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Tell(ref, i)
	}
	b.StopTimer()
}
