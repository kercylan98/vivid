package vivid_test

import (
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
)

func TestActorSystem_ActorOf(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				t.Log(ctx.Ref(), ctx.Sender(), "OnLaunch")
			}
		})
	})
}

func TestActorSystem_Tell(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	wait := make(chan struct{})
	ref := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case int:
				wait <- struct{}{}
			}
		})
	})

	system.Tell(ref, 1)
	<-wait
}

func TestActorSystem_Probe(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	ref := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case int:
				ctx.Reply(1)
			}
		})
	})

	wait := make(chan struct{})
	system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Probe(ref, 1)
			case int:
				wait <- struct{}{}
			}
		})
	})
	<-wait
}

func TestActorSystem_Ask(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	ref := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case int:
				ctx.Reply(1)
			}
		})
	})

	result, err := system.Ask(ref, 1).Result()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result)
}

func TestActorSystem_Kill(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	counter := 100

	ref := system.ActorOf(func() vivid.Actor {
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
	})

	for i := 0; i < counter; i++ {
		system.Tell(ref, i)
	}

	system.Kill(ref)
}

func TestActorSystem_PoisonKill(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	counter := 100
	limit := counter

	ref := system.ActorOf(func() vivid.Actor {
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
	})

	for i := 0; i < limit; i++ {
		system.Tell(ref, i)
	}

	system.PoisonKill(ref)
}
