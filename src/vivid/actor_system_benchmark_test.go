package vivid_test

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
)

func BenchmarkActorSystem_Tell(b *testing.B) {
	system := vivid.NewActorSystem().StartP()

	ref := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Tell(ref, i)
	}
	b.StopTimer()
}

func BenchmarkActorSystem_Probe(b *testing.B) {
	system := vivid.NewActorSystem().StartP()

	ref := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Probe(ref, i)
	}
	b.StopTimer()
}

func BenchmarkActorSystem_Ask(b *testing.B) {
	system := vivid.NewActorSystem(vivid.ActorSystemConfiguratorFN(func(config *vivid.ActorSystemConfig) {
		logger := log.GetBuilder().Silent()
		config.WithLoggerProvider(log.ProviderFn(func() log.Logger {
			return logger
		}))
	})).StartP()
	ref := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			ctx.Reply(ctx.Message())
		})
	})

	var err error

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = system.Ask(ref, i).Result(); err != nil {
			b.Fail()
		}
	}
	b.StopTimer()
}
