package vivid_test

import (
	"errors"
	vivid "github.com/kercylan98/vivid/src"
	"sync"
	"testing"
	"time"
)

func TestAccidentRecord_Kill(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case error:
				panic("accident")
			case string:
				t.Error("should not receive string message")
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithSupervisor(vivid.SupervisorFn(func(record vivid.AccidentRecord) {
			record.Kill(record.GetVictim(), "decision kill")
		}))
	})

	system.Tell(ref, errors.New("hit accident"))
	system.Tell(ref, "string message")
}

func TestAccidentRecord_PoisonKill(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	wait := new(sync.WaitGroup)
	wait.Add(1)

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case error:
				panic("accident")
			case string:
				wait.Done()
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithSupervisor(vivid.SupervisorFn(func(record vivid.AccidentRecord) {
			record.PoisonKill(record.GetVictim(), "decision poison kill")
		}))
	})

	system.Tell(ref, errors.New("hit accident"))
	system.Tell(ref, "string message")

	wait.Wait()
}

func TestAccidentRecord_Resume(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	wait := new(sync.WaitGroup)
	wait.Add(1)

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case error:
				panic("accident")
			case string:
				wait.Done()
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithSupervisor(vivid.SupervisorFn(func(record vivid.AccidentRecord) {
			record.Resume()
		}))
	})

	system.Tell(ref, errors.New("hit accident"))
	system.Tell(ref, "string message")

	wait.Wait()
}

func TestAccidentRecord_Restart(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	wait := new(sync.WaitGroup)
	wait.Add(2)

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case error:
				panic("accident")
			case vivid.OnLaunch:
				if m.Restarted() {
					t.Log("restart")
					wait.Done()
				}
			case string:
				t.Log("string message")
				wait.Done()
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithSupervisor(vivid.SupervisorFn(func(record vivid.AccidentRecord) {
			record.Restart(record.GetVictim(), "decision restart")
		}))
	})

	system.Tell(ref, errors.New("hit accident"))
	system.Tell(ref, "string message")

	wait.Wait()
}

func TestAccidentRecord_Escalate(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	wait := new(sync.WaitGroup)
	wait.Add(1)

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case vivid.OnLaunch:
				if m.Restarted() {
					t.Log("restart")
					wait.Done()
				}
			case error:
				panic("accident")
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithSupervisor(vivid.SupervisorFn(func(record vivid.AccidentRecord) {
			record.Escalate()
		}))
	})

	system.Tell(ref, errors.New("hit accident"))

	wait.Wait()
}

func TestAccidentRecord_ExponentialBackoffRestart(t *testing.T) {
	system := vivid.NewActorSystem().StartP()
	defer system.ShutdownP()

	wait := new(sync.WaitGroup)
	wait.Add(1)

	ref := system.ActorOfFn(func() vivid.Actor {
		return vivid.ActorFn(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case vivid.OnLaunch:
				if m.Restarted() {
					panic("accident")
				}
			case vivid.OnKill:
				wait.Done()
			case error:
				panic("accident")
			}
		})
	}, func(config vivid.ActorConfiguration) {
		config.WithSupervisor(vivid.SupervisorFn(func(record vivid.AccidentRecord) {
			record.ExponentialBackoffRestart(record.GetVictim(), "decision exponential backoff restart", 3, time.Millisecond*100, time.Millisecond*1000, 2, 0.5)
		}))
	})

	system.Tell(ref, errors.New("hit accident"))

	wait.Wait()
}
