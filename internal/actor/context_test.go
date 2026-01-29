package actor_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
)

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
				ref, err := ctx.ActorOf(actor.UselessActor())
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
				actor.UselessActor(),
			))

		assert.ErrorIs(t, err, vivid.ErrorActorSpawnFailed)
		assert.Nil(t, ref)
	})

	t.Run("repeated spawn", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(actor.UselessActor(), vivid.WithActorName("test"))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		ref, err = system.ActorOf(actor.UselessActor(), vivid.WithActorName("test"))
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
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.Become(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case int:
					wg.Done()
					ctx.UnBecome()
					ctx.Tell(ctx.Ref(), 2)
				}
			})
			ctx.Tell(ctx.Ref(), 1)
		case int:
			ctx.UnBecome()
			wg.Done()
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	wg.Wait()
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
}

func TestContext_Watch(t *testing.T) {
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
