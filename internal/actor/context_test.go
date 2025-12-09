package actor_test

import (
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
)

func TestContext_ActorOf(t *testing.T) {
	system := actor.NewSystem()

	var wg sync.WaitGroup
	wg.Add(1)
	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			wg.Done()
		}
	}))

	wg.Wait()
}
