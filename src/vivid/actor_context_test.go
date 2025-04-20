package vivid_test

import (
	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
	"time"
)

func TestActorContext_Watch(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	wait := make(chan struct{})

	system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				child := ctx.ActorOf(func() vivid.Actor {
					return vivid.ActorFN(func(ctx vivid.ActorContext) {})
				})

				ctx.Watch(child)
				ctx.PoisonKill(child)
			case *vivid.OnDead:
				wait <- struct{}{}
			}
		})
	})

	<-wait
}

func TestActorContext_After(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	wait := make(chan struct{})
	system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.After("after", time.Millisecond, timing.TaskFN(func() {
					close(wait)
				}))
			}
		})
	})

	<-wait
}

func TestActorContext_Ping(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	ref := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			return
		})
	})

	pong, err := system.Ping(ref, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Ping RTT: %s", pong.RTT())
	t.Logf("Original time: %v", pong.OriginalTime())
	t.Logf("Response time: %v", pong.ResponseTime())
}
