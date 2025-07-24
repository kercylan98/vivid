package vivid_test

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"sync"
	"testing"
)

func TestActorLaunchHookFN_OnActorLaunch(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	NewTestActorSystem(t, vivid.ActorSystemConfiguratorFN(func(c *vivid.ActorSystemConfiguration) {
		c.WithHooks(vivid.ActorLaunchHookFN(func(ctx vivid.ActorContext) {
			wg.Done()
		}))
	}))
	wg.Wait()
}
