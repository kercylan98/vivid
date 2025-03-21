package vivid_test

import (
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
	"time"
)

func TestActorSystem_ActorOf(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	system.ActorOf(vivid.ActorProviderFN(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				t.Log(ctx.Ref(), ctx.Sender(), "OnLaunch")
			}
		})
	}))

	time.Sleep(time.Second) // TODO: Stop 暂未实现，暂时用 Sleep 代替
}

func TestActorContext_Kill(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	counter := 100

	ref := system.ActorOf(vivid.ActorProviderFN(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnKill:
				if counter == 0 {
					t.Fail()
				}
			case int:
				counter--
			}
		})
	}))

	for i := 0; i < counter; i++ {
		system.Tell(ref, i)
	}

	system.Kill(ref)
	time.Sleep(time.Second) // TODO: Stop 暂未实现，暂时用 Sleep 代替
}

func TestActorContext_PoisonKill(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	counter := 100
	limit := counter

	ref := system.ActorOf(vivid.ActorProviderFN(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnKill:
				if counter != 0 {
					t.Fatalf("counter: %d", counter)
				}
			case int:
				counter--
			}
		})
	}))

	for i := 0; i < limit; i++ {
		system.Tell(ref, i)
	}

	system.PoisonKill(ref)
	time.Sleep(time.Second) // TODO: Stop 暂未实现，暂时用 Sleep 代替
}
