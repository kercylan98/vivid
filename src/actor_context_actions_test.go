package vivid_test

import (
	vivid "github.com/kercylan98/vivid/src"
	"sync"
	"testing"
)

func TestActorContextActionsImpl_Tell(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				t.Log("Receive", m)
			}
		})
	})

	system.Tell(ref, "Hello")
}

func TestActorContextActionsImpl_TellRemote(t *testing.T) {
	system1 := vivid.NewActorSystem().StartP()
	system2 := vivid.NewActorSystem(vivid.ActorSystemConfiguratorFn(func(config vivid.ActorSystemConfiguration) {
		config.WithListen(":8088")
	})).StartP()
	defer system1.ShutdownP()
	defer system2.ShutdownP()

	var wait sync.WaitGroup
	wait.Add(1)

	ref := system2.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				t.Log("Receive", m)
				wait.Done()
			}
		})
	})

	system1.Tell(ref, "Hello")

	wait.Wait()
}

func TestActorContextActionsImpl_Ask(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				t.Log("Receive", m)
				ctx.Reply("World")
			}
		})
	})

	result, err := system.Ask(ref, "Hello").Result()
	if err != nil {
		t.Error(err)
		return
	}

	t.Log("Result", result)
}

func TestActorContextActionsImpl_AskRemote(t *testing.T) {
	system1 := vivid.NewActorSystem().StartP()
	system2 := vivid.NewActorSystem(vivid.ActorSystemConfiguratorFn(func(config vivid.ActorSystemConfiguration) {
		config.WithListen(":8088")
	})).StartP()
	defer system1.ShutdownP()
	defer system2.ShutdownP()

	ref := system2.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				t.Log("Receive", m)
				ctx.Reply("World")
			}
		})
	})

	result, err := system1.Ask(ref, "Hello").Result()
	if err != nil {
		t.Error(err)
		return
	}

	t.Log("Result", result)
}

func TestActorContextActionsImpl_PoisonKill(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	state := 0

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case string:
				if state != 0 {
					t.Error("State is not 0")
					return
				}
				state = 1
				t.Log("Receive", m)
			case vivid.OnKill:
				if state != 1 {
					t.Error("State is not 1")
					return
				}
				state = 2
			}
		})
	})

	system.Tell(ref, "Hello")
	system.PoisonKill(ref)
}
