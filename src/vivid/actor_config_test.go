package vivid_test

import (
	"errors"
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid"
	"testing"
)

func TestActorConfig_WithSupervisor(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.StopP()

	ref := system.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Logger().Info("Actor started", log.Bool("restart", m.Restarted()))
			case error:
				panic(m)
			}
		})
	}, func(config *vivid.ActorConfig) {
		config.WithSupervisor(vivid.SupervisorFN(func(snapshot vivid.AccidentSnapshot) {
			snapshot.Restart(snapshot.GetVictim(), "must restart")
		}))
	})

	system.Tell(ref, errors.New("down"))
}
