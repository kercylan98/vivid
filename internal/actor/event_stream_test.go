package actor_test

import (
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
)

func TestEventStream_Subscribe(t *testing.T) {
	type TestEvent struct{}

	t.Run("repeated subscribe", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()
		system.EventStream().Subscribe(system, ves.ActorSpawnedEvent{})
		system.EventStream().Subscribe(system, ves.ActorSpawnedEvent{})
	})

	t.Run("unsubscribe", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.EventStream().Subscribe(ctx, TestEvent{})
				ctx.EventStream().Unsubscribe(ctx, TestEvent{})
				ctx.EventStream().Publish(ctx, TestEvent{})

				// unsubscribe not subscribed event
				ctx.EventStream().Unsubscribe(ctx, 1)
			case TestEvent:
				panic("test event received")
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)

	})

}
