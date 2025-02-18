package vivid_test

import (
	vivid "github.com/kercylan98/vivid/src"
	"testing"
)

func TestActorContextLifeImpl_System(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case vivid.OnLaunch:
				if ctx.System() != system {
					t.Error("ActorContext.System() should return the system that the actor belongs to")
				}
			}
		})
	})
}

func TestActorContextLifeImpl_Parent(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case vivid.OnLaunch:
				if !ctx.Parent().Equal(system.Ref()) {
					t.Error("ActorContext.Parent() should return the parent actor reference")
				}
			}
		})
	})
}
