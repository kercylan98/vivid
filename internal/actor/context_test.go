package actor_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/internal/sugar"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
)

func TestContext_Supervision(t *testing.T) {
	t.Run("one for all graceful stop", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var childCount = 10
		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case *vivid.OnLaunch:
				for i := 0; i < childCount; i++ {
					child, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
						switch ctx.Message().(type) {
						case *vivid.OnLaunch:
							if i == childCount-1 {
								ctx.Failed("test error")
							}
						}
					}))
					assert.NoError(t, err)
					assert.NotNil(t, child)
				}
			case *vivid.OnKilled:
				if m.Ref.Equals(ctx.Ref()) {
					close(wait)
				} else {
					childCount--
					ctx.Logger().Debug("test counter", log.Int("child_count", childCount))
					if childCount == 0 {
						ctx.Kill(ctx.Ref(), false, "all children killed")
					}
				}
			}
		}), vivid.WithActorSupervisionStrategy(vivid.OneForAllStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
			return vivid.SupervisionDecisionGracefulStop, "graceful stop"
		}))))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
			assert.Equal(t, 0, childCount)
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("one for all graceful restart", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var childCount atomic.Int32
		childCount.Store(10)
		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				for i := 0; i < int(childCount.Load()); i++ {
					child, err := ctx.ActorOf(vivid.NewRestartedActor(func(ctx vivid.RestartContext) error {
						childCount.Add(-1)
						if childCount.Load() == 0 {
							close(wait)
						}
						return nil
					}, vivid.ActorFN(func(ctx vivid.ActorContext) {
						switch ctx.Message().(type) {
						case *vivid.OnLaunch:
							if i == int(childCount.Load())-1 {
								ctx.Failed("test error")
							}
						}
					})))
					assert.NoError(t, err)
					assert.NotNil(t, child)
				}
			}
		}), vivid.WithActorSupervisionStrategy(vivid.OneForAllStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
			return vivid.SupervisionDecisionGracefulRestart, "graceful restart"
		}))))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
			assert.Equal(t, int32(0), childCount.Load())
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("one for one resume", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				child, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch ctx.Message().(type) {
					case *vivid.OnLaunch:
						ctx.Tell(ctx.Ref(), "resume message")
						ctx.Failed(errors.New("test error"))
					case string:
						close(wait)
					}
				}))
				assert.NoError(t, err)
				assert.NotNil(t, child)
			}
		}), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
			return vivid.SupervisionDecisionResume, "resume"
		}))))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("one for one escalate", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				child, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch ctx.Message().(type) {
					case *vivid.OnLaunch:
						ctx.Failed(errors.New("test error"))
					case *vivid.OnKilled:
						close(wait)
					}
				}))
				assert.NoError(t, err)
				assert.NotNil(t, child)
			}
		}), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
			return vivid.SupervisionDecisionEscalate, "escalate"
		}))))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("basic function", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait1 = make(chan struct{})
		var wait2 = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				child1, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch ctx.Message().(type) {
					case *vivid.OnLaunch:
						child2, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
							switch ctx.Message().(type) {
							case *vivid.OnLaunch:
								ctx.Failed(errors.New("test error"))
							case *vivid.OnKilled:
								close(wait2)
							}
						}))
						assert.NoError(t, err)
						assert.NotNil(t, child2)
					case *vivid.OnKilled:
						close(wait1)
					}
				}), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
					return vivid.SupervisionDecisionEscalate, "test escalate"
				}))))
				assert.NoError(t, err)
				assert.NotNil(t, child1)
			}
		}), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
			ctx.Logger().Debug("supervision basic",
				log.String("reason", reason),
				log.Any("fault", ctx.Fault()),
				log.Any("fault_stack", string(ctx.FaultStack())),
			)
			return vivid.SupervisionDecisionStop, "test stop"
		}))))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-sugar.WaitAllChannel(wait1, wait2):
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})
}

func TestContext_Command(t *testing.T) {
	t.Run("pause and resume mailbox", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(actor.NewUselessActor())
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		_, _ = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.EventStream().Subscribe(ctx, ves.ActorMailboxPausedEvent{})
				ctx.EventStream().Subscribe(ctx, ves.ActorMailboxResumedEvent{})

				system.AdvancedTell(true, ref, messages.CommandPauseMailbox.Build())
			case ves.ActorMailboxPausedEvent:
				ctx.Logger().Info("receive mailbox paused event")
				system.AdvancedTell(true, ref, messages.CommandResumeMailbox.Build())
			case ves.ActorMailboxResumedEvent:
				ctx.Logger().Info("receive mailbox resumed event")
				close(wait)
			}
		}))

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})
}

func TestContext_Restart(t *testing.T) {
	t.Run("restart", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var counter = 0
		var wait = make(chan struct{})
		ref, err := system.ActorOf(
			vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case *vivid.OnLaunch:
					childRef, err := ctx.ActorOf(vivid.NewPreRestartActor(func(ctx vivid.RestartContext) error {
						ctx.Logger().Info("pre restart")
						return nil
					}, vivid.ActorFN(func(ctx vivid.ActorContext) {
						switch ctx.Message().(type) {
						case *vivid.OnLaunch:
							counter++
							switch counter {
							case 1:
								ctx.Failed("test error")
							case 2:
								close(wait)
							}
						}
					})))
					assert.NoError(t, err)
					assert.NotNil(t, childRef)
				}
			}),
			vivid.WithActorSupervisionStrategy(
				vivid.OneForOneStrategy(
					vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
						return vivid.SupervisionDecisionRestart, "test restart"
					}),
				),
			),
		)
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("restart on pre launch error", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				var childPanicked bool
				childActor := vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
					if !childPanicked {
						return nil
					}
					return errors.New("test pre launch error")
				}, vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch ctx.Message().(type) {
					case *vivid.OnLaunch:
						if !childPanicked {
							childPanicked = true
							panic("test pre launch panic")
						}
					}
				}))

				childRef, err := ctx.ActorOf(childActor)
				assert.NoError(t, err)
				assert.NotNil(t, childRef)
			}
		}), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
			return vivid.SupervisionDecisionRestart, "test restart"
		}))))
		assert.NoError(t, err)
		assert.NotNil(t, ref)
	})

	t.Run("restart on kill panic", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var launchPanicked bool
		var killPanicked bool
		var wait = make(chan struct{})
		parentRef, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				childRef, err := ctx.ActorOf(vivid.NewPreRestartActor(func(ctx vivid.RestartContext) error {
					close(wait)
					return nil
				}, vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch ctx.Message().(type) {
					case *vivid.OnLaunch:
						if !launchPanicked {
							launchPanicked = true
							panic("child on launch panic")
						}

						ctx.Logger().Info("child on launch ok")
					case *vivid.OnKill:
						if !killPanicked {
							killPanicked = true
							panic("child on kill panic")
						}

						ctx.Logger().Info("child on kill ok")
					}
				})))
				assert.NoError(t, err)
				assert.NotNil(t, childRef)
			}
		}), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
			return vivid.SupervisionDecisionRestart, "test restart"
		}))))
		assert.NoError(t, err)
		assert.NotNil(t, parentRef)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("restart on killed panic, and prelaunch", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var launchPanicked bool
		var killPanicked bool
		var wait = make(chan struct{})
		parentRef, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				childActor := vivid.NewComplexCombinationActor(
					vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
						return nil
					}),
					vivid.NewPreRestartActor(func(ctx vivid.RestartContext) error {
						close(wait)
						return nil
					}, vivid.ActorFN(func(ctx vivid.ActorContext) {
						switch ctx.Message().(type) {
						case *vivid.OnLaunch:
							if !launchPanicked {
								launchPanicked = true
								panic("child on launch panic")
							}

							ctx.Logger().Info("child on launch ok")
						case *vivid.OnKilled:
							if !killPanicked {
								killPanicked = true
								panic("child on killed panic")
							}

							ctx.Logger().Info("child on killed ok")
						}
					}),
					))

				childRef, err := ctx.ActorOf(childActor)
				assert.NoError(t, err)
				assert.NotNil(t, childRef)
			}
		}), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
			return vivid.SupervisionDecisionRestart, "test restart"
		}))))
		assert.NoError(t, err)
		assert.NotNil(t, parentRef)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("with provider", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		var newActor = func() vivid.Actor {
			var counter = 0
			return vivid.NewRestartedActor(func(ctx vivid.RestartContext) error {
				if counter != 0 {
					assert.Fail(t, "counter actor count is not 0")
				}
				close(wait)
				return nil
			}, vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch v := ctx.Message().(type) {
				case *vivid.OnLaunch:
				case int:
					counter += v
					if v == 10 {
						panic("counter actor panic")
					}
				}
			}))
		}

		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ref, err := ctx.ActorOf(newActor(), vivid.WithActorProvider(vivid.ActorProviderFN(newActor)))
				assert.NoError(t, err)
				assert.NotNil(t, ref)
				ctx.Tell(ref, 10)
			}
		}), vivid.WithActorSupervisionStrategy(vivid.OneForOneStrategy(vivid.SupervisionStrategyDecisionMakerFN(func(ctx vivid.SupervisionContext) (decision vivid.SupervisionDecision, reason string) {
			return vivid.SupervisionDecisionRestart, "test with provider restart"
		}))))

		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
			time.Sleep(time.Second)
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})
}

func TestContext_Failed(t *testing.T) {

	t.Run("failed", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Failed(errors.New("on launch error"))
			case *vivid.OnKilled:
				close(wait)
			}
		}))

		assert.NoError(t, err)
		assert.NotNil(t, ref)
		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("killing failed", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnKill:
				ctx.Failed(errors.New("test error"))
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		system.Kill(ref, false)
	})

	t.Run("killing panic", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnKill:
				panic("test panic")
			case *vivid.OnKilled:
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		system.Kill(ref, false)
		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("killed panic", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnKilled:
				panic("test panic")
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		system.Kill(ref, false)
	})

	t.Run("handler child killed failed", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case *vivid.OnLaunch:
				childRef, err := ctx.ActorOf(actor.NewUselessActor())
				assert.NoError(t, err)
				assert.NotNil(t, childRef)

				system.Kill(childRef, false)
			case *vivid.OnKilled:
				if !m.Ref.Equals(ctx.Ref()) {
					panic("child killed not self")
				} else {
					close(wait)
				}
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})
}

func TestContext_Children(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var n = 10
	for i := 0; i < n; i++ {
		ref, err := system.ActorOf(actor.NewUselessActor())
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		assert.Equal(t, i+1, system.Children().Len())
	}
}

func TestContext_Name(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var wait = make(chan struct{})
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			assert.Equal(t, "name", ctx.Name())
			wait <- struct{}{}
		}
	}), vivid.WithActorName("name"))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	select {
	case <-wait:
	case <-time.After(time.Second):
		assert.Fail(t, "timeout")
	}
}

func TestContext_System(t *testing.T) {
	system := actor.NewSystem()
	assert.NoError(t, system.Start())
	defer func() {
		assert.NoError(t, system.Stop())
	}()
	wait := make(chan struct{})
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			assert.Equal(t, ctx.System(), system)
			wait <- struct{}{}
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)
	select {
	case <-wait:
	case <-time.After(1 * time.Second):
		assert.Fail(t, "timeout")
	}
}

func TestContext_StashCount(t *testing.T) {
	system := actor.NewTestSystem(t)

	var first bool
	var wait = make(chan struct{})
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			if !first {
				first = true
				ctx.Stash()
				assert.Equal(t, ctx.StashCount(), 1)
				ctx.Unstash()
				assert.Equal(t, ctx.StashCount(), 0)
			} else {
				wait <- struct{}{}
			}
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	select {
	case <-wait:
	case <-time.After(1 * time.Second):
		assert.Fail(t, "timeout")
	}
}

func TestContext_Unstash(t *testing.T) {
	type Stash struct{}
	type Unstash struct{}
	var cases = []struct {
		name  string
		actor func(stashed *bool, wait chan<- struct{}) vivid.Actor
	}{
		{name: "unstash", actor: func(stashed *bool, wait chan<- struct{}) vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case Stash:
					if !*stashed {
						*stashed = true
						ctx.Stash()
					} else {
						wait <- struct{}{}
					}
				case Unstash:
					ctx.Unstash(1)
				}
			})
		}},

		{name: "fast unstash", actor: func(stashed *bool, wait chan<- struct{}) vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case Stash:
					if !*stashed {
						*stashed = true
						ctx.Stash()
					} else {
						wait <- struct{}{}
					}
				case Unstash:
					ctx.Unstash()
				}
			})
		}},

		{name: "batch unstash", actor: func(stashed *bool, wait chan<- struct{}) vivid.Actor {
			count := 0
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case Stash:
					if !*stashed {
						*stashed = true
						ctx.Stash()
						ctx.Stash()
					} else {
						count++
						if count == 2 {
							wait <- struct{}{}
						}
					}
				case Unstash:
					ctx.Unstash(3) // auto fix to 2
				}
			})
		}},

		{name: "empty unstash", actor: func(stashed *bool, wait chan<- struct{}) vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case Unstash:
					ctx.Unstash()
					wait <- struct{}{}
				}
			})
		}},
	}

	for _, s := range cases {
		t.Run(s.name, func(t *testing.T) {
			system := actor.NewTestSystem(t)
			defer func() {
				assert.NoError(t, system.Stop())
			}()

			var stashed bool
			var wait = make(chan struct{})

			ref, err := system.ActorOf(s.actor(&stashed, wait))

			assert.NoError(t, err)
			assert.NotNil(t, ref)

			system.Tell(ref, Stash{})
			system.Tell(ref, Unstash{})

			select {
			case <-wait:
			case <-time.After(time.Second):
				assert.Fail(t, "stash timeout")
			}
		})
	}
}

func TestContext_ActorOf(t *testing.T) {
	t.Run("actor_of", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wg sync.WaitGroup
		wg.Add(1)
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				wg.Done()
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		wg.Wait()
	})

	t.Run("killed after", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnKilled:
				ref, err := ctx.ActorOf(actor.NewUselessActor())
				assert.ErrorIs(t, err, vivid.ErrorActorDeaded)
				assert.Nil(t, ref)
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		system.Kill(ref, false)
		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("spawn failed", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(
			vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
				return errors.New("test error")
			},
				actor.NewUselessActor(),
			))

		assert.ErrorIs(t, err, vivid.ErrorActorSpawnFailed)
		assert.Nil(t, ref)
	})

	t.Run("repeated spawn", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(actor.NewUselessActor(), vivid.WithActorName("test"))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		ref, err = system.ActorOf(actor.NewUselessActor(), vivid.WithActorName("test"))
		assert.ErrorIs(t, err, vivid.ErrorActorAlreadyExists)
		assert.Nil(t, ref)
	})

	t.Run("killing spawn", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnKill:
				ref, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch ctx.Message().(type) {
					case *vivid.OnKilled:
						close(wait)
					}
				}))
				assert.NoError(t, err)
				assert.NotNil(t, ref)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		system.Kill(ref, false)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})
}

func TestContext_Become(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.Become(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case int:
					wg.Done()
				}
			})
			ctx.Tell(ctx.Ref(), 1)
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	wg.Wait()
}

func TestContext_RevertBehavior(t *testing.T) {
	t.Run("unbecome", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Become(func(ctx vivid.ActorContext) {})
				ctx.Become(func(ctx vivid.ActorContext) {})
				ctx.Become(func(ctx vivid.ActorContext) {})
				ctx.UnBecome()
			case int:
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		system.Tell(ref, 1)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("not discard old", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Become(func(ctx vivid.ActorContext) {})
				ctx.Become(func(ctx vivid.ActorContext) {})
				ctx.UnBecome(vivid.WithBehaviorDiscardOld(false))
			case int:
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		system.Tell(ref, 1)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})
}

func TestContext_Receive(t *testing.T) {
	t.Run("panic", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case error:
				panic(m)
			case *vivid.OnKilled: // 触发默认监管策略后杀死
				close(wait)
			}
		}))

		assert.NoError(t, err)
		assert.NotNil(t, ref)
		system.Tell(ref, errors.New("test panic"))
		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})
}

func TestContext_Tell(t *testing.T) {
	t.Run("tell", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wg sync.WaitGroup
		wg.Add(1)
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case int:
				wg.Done()
			}
		}))
		assert.NoError(t, err)

		system.Tell(ref, 1)
		wg.Wait()
	})

	t.Run("tell nil ref", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		system.Tell(nil, 0)
	})
}

func BenchmarkContext_Tell(b *testing.B) {
	system := actor.NewSystem()
	if err := system.Start(); err != nil {
		b.Fatal(err)
	}

	defer func() {
		if err := system.Stop(); err != nil {
			b.Fatal(err)
		}
	}()

	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
	if err != nil {
		b.Fatal(err)
	}

	message := struct{}{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Tell(ref, message)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func TestContext_Ask(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var wg sync.WaitGroup
	wg.Add(3)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case int:
			ctx.Reply(1)
			wg.Done()
		}
	}))
	assert.NoError(t, err)

	_, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			reply, err := system.Ask(ref, 1).Result()
			assert.Nil(t, err)
			assert.Equal(t, 1, reply.(int))
			wg.Done()
		}
	}))
	assert.NoError(t, err)

	systemReply, systemErr := system.Ask(ref, 1).Result()
	assert.Nil(t, systemErr)
	assert.Equal(t, 1, systemReply.(int))

	wg.Wait()
}

func BenchmarkContext_Ask(b *testing.B) {
	system := actor.NewSystem()
	if err := system.Start(); err != nil {
		b.Fatal(err)
	}

	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case int:
			ctx.Reply(message)
		}
	}))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := system.Ask(ref, i).Result(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportAllocs()
}

func TestContext_Sender(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var refAHeldRefB vivid.ActorRef
	var wg sync.WaitGroup
	wg.Add(1)

	refA, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch m := ctx.Message().(type) {
		case vivid.ActorRef:
			refAHeldRefB = m
		case string:
			assert.Equal(t, refAHeldRefB.GetAddress()+refAHeldRefB.GetPath(), ctx.Sender().GetAddress()+ctx.Sender().GetPath(), "sender ref mismatch")
			wg.Done()
		}
	}))
	assert.NoError(t, err)

	refB, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case string:
			ctx.Ask(refA, "ask-ref-b")
		}
	}))
	assert.NoError(t, err)

	system.Tell(refA, refB)
	system.Tell(refB, "start-ask")
	wg.Wait()
}

func TestContext_Kill(t *testing.T) {
	t.Run("kill", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()
		var wg sync.WaitGroup
		wg.Add(5)
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ref, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch ctx.Message().(type) {
					case *vivid.OnKill, *vivid.OnKilled:
						wg.Done()
					}
				}))
				assert.NoError(t, err)
				assert.NotNil(t, ref)
			case *vivid.OnKill, *vivid.OnKilled:
				wg.Done()
			}
		}))
		assert.NoError(t, err)

		system.Kill(ref, false, "test kill")
		wg.Wait()
	})

	t.Run("repeated kill", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(actor.NewUselessActor())
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		system.Kill(ref, false, "test kill")
		system.Kill(ref, false, "test kill")
		system.Kill(ref, false, "test kill")
	})

	t.Run("repeated kill, actor state is killing", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		var childCanKill = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				childRef, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch ctx.Message().(type) {
					case *vivid.OnKill:
						<-childCanKill
					}
				}))
				assert.NoError(t, err)
				assert.NotNil(t, childRef)
			case *vivid.OnKill:
				close(childCanKill)
			case *vivid.OnKilled:
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		system.Kill(ref, false, "test kill")
		system.Kill(ref, false, "test kill")

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})
}

func TestContext_Unwatch(t *testing.T) {
	t.Run("unwatch", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(actor.NewUselessActor())
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		var wait = make(chan struct{})
		watcherRef, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Watch(ref)
				ctx.Unwatch(ref)
				ctx.Kill(ref, false)
			case *vivid.OnKilled:
				if m.Ref.Equals(ref) {
					assert.Fail(t, "watcher ref mismatch")
				} else {
					time.Sleep(100 * time.Millisecond) // 等待可能的异步处理
					close(wait)
				}
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, watcherRef)
		system.Kill(watcherRef, true)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("not watch", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(actor.NewUselessActor())
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		system.Unwatch(ref)
	})
}

func TestContext_Watch(t *testing.T) {
	t.Run("watch", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		var wg sync.WaitGroup
		wg.Add(2)
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
		assert.NoError(t, err)

		_, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch message := ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Watch(ref)
				ctx.Kill(ref, false, "test kill")
				wg.Done()
			case *vivid.OnKilled:
				if message.Ref.Equals(ref) {
					wg.Done()
				}
			}
		}))
		assert.NoError(t, err)

		wg.Wait()
	})

	t.Run("watch child", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ref, err := ctx.ActorOf(actor.NewUselessActor())
				assert.NoError(t, err)
				assert.NotNil(t, ref)
				ctx.Watch(ref)
			}
		}))

		assert.NoError(t, err)
		assert.NotNil(t, ref)
	})

	t.Run("repeated watch", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(actor.NewUselessActor())

		var wait = make(chan struct{})
		ref2, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Watch(ref)
				ctx.Watch(ref)
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref2)

		select {
		case <-wait:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout")
		}
	})

}

func TestContext_DeathLetter(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var waitSub = make(chan struct{})
	var waitKilled = make(chan struct{})
	var waitDeathLetter = make(chan struct{})

	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.EventStream().Subscribe(ctx, ves.DeathLetterEvent{})
			close(waitSub)
		case ves.DeathLetterEvent:
			close(waitDeathLetter)
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	<-waitSub
	ref, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.Kill(ctx.Ref(), true, "test kill")
		case *vivid.OnKilled:
			close(waitKilled)
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	<-waitKilled
	system.Tell(ref, "test message")
	<-waitDeathLetter
}

func TestContext_Entrust(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	t.Run("invalid task", func(t *testing.T) {
		result, err := system.Entrust(-1, nil).Result()
		assert.NotNil(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, vivid.ErrorFutureInvalid))
	})

	t.Run("valid task", func(t *testing.T) {
		result, err := system.Entrust(-1, vivid.EntrustTaskFN(func() (vivid.Message, error) {
			return true, nil
		})).Result()
		assert.Nil(t, err)
		assert.True(t, result.(bool))
	})

	t.Run("err task", func(t *testing.T) {
		result, err := system.Entrust(-1, vivid.EntrustTaskFN(func() (vivid.Message, error) {
			return nil, errors.New("test error")
		})).Result()
		assert.NotNil(t, err)
		assert.Nil(t, result)
	})

	t.Run("task recover err", func(t *testing.T) {
		var err = errors.New("test panic error")
		result, err := system.Entrust(-1, vivid.EntrustTaskFN(func() (vivid.Message, error) {
			panic(err)
		})).Result()
		assert.NotNil(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, err))
		assert.True(t, errors.Is(err, vivid.ErrorFutureUnexpectedError))
	})

	t.Run("task recover panic", func(t *testing.T) {
		result, err := system.Entrust(-1, vivid.EntrustTaskFN(func() (vivid.Message, error) {
			panic("test panic")
		})).Result()
		assert.NotNil(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, vivid.ErrorFutureUnexpectedError))
	})
}

func TestContext_PipeTo(t *testing.T) {

	type Query struct{ Text string }
	type Response struct{ Text string }

	t.Run("pipe to", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan Response)
		var provider = vivid.ActorProviderFN(func() vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch m := ctx.Message().(type) {
				case Query:
					ctx.Reply(Response{Text: m.Text})
				case *vivid.PipeResult:
					wait <- m.Message.(Response)
				}
			})
		})

		var refs = make(vivid.ActorRefs, 0)
		for i := 0; i < 10; i++ {
			ref, err := system.ActorOf(provider.Provide())
			assert.NoError(t, err)
			assert.NotNil(t, ref)
			refs = append(refs, ref)
		}

		pipeId := system.PipeTo(refs.Rand(), Query{Text: "test"}, refs, 1*time.Second)
		assert.NotEmpty(t, pipeId)

		var result []Response
	loop:
		for {
			select {
			case v := <-wait:
				result = append(result, v)
				if len(result) == refs.Len() {
					break loop
				}
				continue
			case <-time.After(1 * time.Second):
				assert.Fail(t, "timeout")
			}
		}
		assert.Equal(t, refs.Len(), len(result))
		for _, r := range result {
			assert.Equal(t, "test", r.Text)
		}
	})

	t.Run("none forwarders", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch v := ctx.Message().(type) {
			case Query:
				ctx.Reply(Response{Text: v.Text})
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		pipeId := system.PipeTo(ref, Query{Text: "test"}, nil, 1*time.Second)
		assert.NotEmpty(t, pipeId)
	})

	t.Run("forward to self", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wait = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case *vivid.OnLaunch:
				pipeId := ctx.PipeTo(ctx.Ref(), Query{Text: "test"}, ctx.Ref().ToActorRefs(), 1*time.Second)
				assert.NotEmpty(t, pipeId)
			case Query:
				ctx.Reply(Response{Text: m.Text})
			case *vivid.PipeResult:
				assert.Equal(t, "test", m.Message.(Response).Text)
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
		case <-time.After(1 * time.Second):
			assert.Fail(t, "timeout")
		}
	})
}

func TestContext_Ping(t *testing.T) {
	t.Run("ping", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(actor.NewUselessActor())
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		pong, err := system.Ping(ref)
		assert.NoError(t, err)
		assert.NotNil(t, pong)
	})

	t.Run("remote ping", func(t *testing.T) {
		system1 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8080"))
		system2 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8081"))
		defer func() {
			assert.NoError(t, system1.Stop())
			assert.NoError(t, system2.Stop())
		}()

		pong, err := system1.Ping(system2.Ref())
		assert.NoError(t, err)
		assert.NotNil(t, pong)
		system1.Logger().Info("pong", "pong", pong)
	})

	t.Run("timeout", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		var wake = make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				<-wake
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		pong, err := system.Ping(ref)
		assert.ErrorIs(t, err, vivid.ErrorFutureTimeout)
		assert.Nil(t, pong)
		close(wake)
	})
}
