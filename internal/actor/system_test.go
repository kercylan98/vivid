package actor_test

import (
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
)

func TestSystem_Stop(t *testing.T) {
	system := actor.NewSystem().Unwrap()

	var wg sync.WaitGroup
	wg.Add(3)
	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			wg.Done()
		case *vivid.OnKill, *vivid.OnKilled:
			wg.Done()
		}
	}))

	system.Stop()
	wg.Wait()
}
