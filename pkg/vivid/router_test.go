package vivid_test

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"testing"
)

func TestActorRouter(t *testing.T) {
	const num = 10
	system := NewTestActorSystem(t).WaitAdd(num)
	defer system.Shutdown(true)
	routerRef := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch context.Message().(type) {
			case int:
				t.Logf("%s receive %d", context.Ref().String(), context.Message())
				system.WaitDone()
			}
		})
	}).WithConfigurators(vivid.ActorConfiguratorFN(func(c *vivid.ActorConfiguration) {
		c.WithName("router")
		c.WithRouter(num, vivid.NewRoundRobinRouterSelector())
	}))

	for i := 0; i < num; i++ {
		system.Tell(routerRef, i)
	}

	system.Wait()
}
