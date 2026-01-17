package actor_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
)

func TestContext_ActorOf(t *testing.T) {
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
					// self kill, self killed
					wg.Done()
				}
			}))
			assert.NoError(t, err)
			assert.NotNil(t, ref)
		case *vivid.OnKill, *vivid.OnKilled:
			// self kill, child killed, self killed
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
}
