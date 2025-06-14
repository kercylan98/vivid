package vivid_test

import (
	"errors"
	"github.com/kercylan98/vivid/core/vivid"
	"testing"
	"time"
)

func TestSupervisor(t *testing.T) {
	system := NewTestActorSystem(t)
	ref := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch m := context.Message().(type) {
			case *vivid.OnLaunch:
				t.Log("on launch")
			case *vivid.OnPreRestart:
				t.Log("on pre restart")
				panic(m)
			case *vivid.OnRestart:
				t.Log("on restart")
				panic(m)
			case *vivid.OnKill:
				t.Log("on kill")
			case error:
				panic(m)
			}
		})
	}).WithConfigurators(vivid.ActorConfiguratorFN(func(c *vivid.ActorConfiguration) {
		c.WithSupervisionProvider(vivid.SupervisorProviderFN(func() vivid.Supervisor {
			return vivid.StandardSupervisorWithConfigurators(vivid.StandardSupervisorConfiguratorFN(func(c *vivid.StandardSupervisorConfiguration) {
				c.WithBackoffMaxRetries(5)
			}))
		}))
	}))

	system.Tell(ref, errors.New("GG"))

	time.Sleep(time.Minute * 10)
}
