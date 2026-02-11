package actor_test

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
)

func TestScheduler_Remoting(t *testing.T) {
	t.Run("remoting", func(t *testing.T) {
		system1 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8080"))
		defer func() {
			assert.NoError(t, system1.Stop())
		}()
		system2 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8081"))
		defer func() {
			assert.NoError(t, system2.Stop())
		}()

		wait := make(chan struct{})
		ref, err := system1.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *actor.TestRemoteMessage:
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		err = system2.Scheduler().Once(ref.Clone(), time.Millisecond, &actor.TestRemoteMessage{Text: "hello"})
		assert.NoError(t, err)

		select {
		case <-wait:
		case <-time.After(time.Second * 3):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("encoding fail", func(t *testing.T) {
		system1 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8080"))
		defer func() {
			assert.NoError(t, system1.Stop())
		}()
		system2 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8081"))
		defer func() {
			assert.NoError(t, system2.Stop())
		}()

		wait := make(chan struct{})
		receiver, err := system1.ActorOf(actor.NewUselessActor())
		assert.NoError(t, err)
		assert.NotNil(t, receiver)

		watcher, err := system2.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.EventStream().Subscribe(ctx, ves.RemotingMessageSendFailedEvent{})
			case ves.RemotingMessageSendFailedEvent:
				if assert.ErrorIs(t, m.Error, vivid.ErrorRemotingMessageEncodeFailed) {
					close(wait)
				}
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, watcher)

		err = system2.Scheduler().Once(receiver.Clone(), time.Millisecond, &actor.TestRemoteMessage{Text: "hello", WriteCodecFail: true})
		assert.NoError(t, err)

		select {
		case <-wait:
		case <-time.After(time.Second * 3):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("decoding fail", func(t *testing.T) {
		system1 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8080"))
		defer func() {
			assert.NoError(t, system1.Stop())
		}()
		system2 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8081"))
		defer func() {
			assert.NoError(t, system2.Stop())
		}()

		wait := make(chan struct{})
		receiver, err := system1.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch m := ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.EventStream().Subscribe(ctx, ves.RemotingMessageDecodeFailedEvent{})
			case ves.RemotingMessageDecodeFailedEvent:
				if assert.ErrorIs(t, m.Error, vivid.ErrorRemotingMessageDecodeFailed) {
					close(wait)
				}
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, receiver)

		err = system2.Scheduler().Once(receiver.Clone(), time.Millisecond, &actor.TestRemoteMessage{Text: "hello", ReadCodecFail: true})
		assert.NoError(t, err)

		select {
		case <-wait:
		case <-time.After(time.Second * 3):
			assert.Fail(t, "timeout")
		}
	})
}

func TestScheduler_Clear(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	system.Scheduler().Clear()

	err := system.Scheduler().Loop(system.Ref(), 1*time.Second, 1)
	assert.NoError(t, err)
	system.Scheduler().Clear()
}

func TestScheduler_Once(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	wait := make(chan struct{})
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.Scheduler().Once(ctx.Ref(), 100*time.Millisecond, 1)
		case int:
			close(wait)
		}
	}))

	assert.NoError(t, err)
	assert.NotNil(t, ref)

	select {
	case <-wait:
	case <-time.After(time.Second * 3):
		assert.Fail(t, "timeout")
	}
}

func TestScheduler_Loop(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	const stopCount = 3
	const interval = 100 * time.Millisecond
	var count int32
	wait := make(chan struct{})
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			assert.NoError(t, actor.NewTestSystem(t).Scheduler().Loop(ctx.Ref(), interval, 1))
		case int:
			count++
			ctx.Logger().Debug("scheduler received", log.String("reference", "test"), log.Time("time", time.Now()), log.String("messageType", "int"))
			if count == stopCount {
				ctx.Scheduler().Clear()
				close(wait)
			}
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	select {
	case <-wait:
	case <-time.After(time.Second * 3):
		assert.Fail(t, "timeout")
	}
}

func TestScheduler_Cron(t *testing.T) {
	t.Run("cron", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		wait := make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.Scheduler().Cron(ctx.Ref(), "* * * * * *", 1)
			case int:
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
		case <-time.After(time.Second * 3):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("cron error", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		assert.ErrorIs(t, system.Scheduler().Cron(system.Ref(), "invalid", 1), vivid.ErrorCronParse)
	})
}

func TestScheduler_Cancel(t *testing.T) {
	t.Run("cancel", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		wait := make(chan struct{})
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				assert.NoError(t, ctx.Scheduler().Once(ctx.Ref(), 100*time.Millisecond, 1, vivid.WithSchedulerReference("test")))
				assert.NoError(t, ctx.Scheduler().Cancel("test"))
				assert.NoError(t, ctx.Scheduler().Once(ctx.Ref(), 300*time.Millisecond, true))
			case bool:
				close(wait)
			case int:
				assert.Fail(t, "should not receive int")
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

		select {
		case <-wait:
		case <-time.After(time.Second * 3):
			assert.Fail(t, "timeout")
		}
	})

	t.Run("cancel not found", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		assert.ErrorIs(t, system.Scheduler().Cancel("none"), vivid.ErrorNotFound)
	})
}

func TestScheduler_Exists(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		assert.False(t, system.Scheduler().Exists("none"))
		system.Scheduler().Once(system.Ref(), 1000*time.Millisecond, 1, vivid.WithSchedulerReference("test"))
		assert.True(t, system.Scheduler().Exists("test"))
		system.Scheduler().Cancel("test")
		assert.False(t, system.Scheduler().Exists("test"))
	})
}
