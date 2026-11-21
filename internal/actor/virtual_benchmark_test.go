package actor

import (
	"strconv"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

func benchmarkVirtualSystem(b *testing.B) *System {
	b.Helper()
	system := NewSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(int); ok {
					ctx.Reply(1)
				}
			})
		})),
	)
	if err := system.Start(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := system.Stop(30 * time.Second); err != nil {
			b.Fatal(err)
		}
	})
	return system
}

func BenchmarkVirtualTell(b *testing.B) {
	system := benchmarkVirtualSystem(b)
	ref := system.VirtualRef("worker", "one")
	if err := system.Ask(ref, 1).Wait(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		system.Tell(ref, struct{}{})
	}
	b.StopTimer()
	// 等待同一普通队列的屏障，确保消息被消费，避免遗留到下一次采样。
	if err := system.Ask(ref, 1, 30*time.Second).Wait(); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkVirtualAsk(b *testing.B) {
	system := benchmarkVirtualSystem(b)
	ref := system.VirtualRef("worker", "one")
	if err := system.Ask(ref, 1).Wait(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := system.Ask(ref, 1).Wait(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVirtualActivate(b *testing.B) {
	system := benchmarkVirtualSystem(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		if err := system.Ask(system.VirtualRef("worker", strconv.Itoa(i)), 1).Wait(); err != nil {
			b.Fatal(err)
		}
	}
}
