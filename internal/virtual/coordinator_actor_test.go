package virtual_test

import (
	"fmt"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
)

type IncrementMessage struct {
	count int64
}

type GetCounterMessage struct {
}

type CounterVirtualActorMock struct {
	counter int64
}

func (v *CounterVirtualActorMock) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *IncrementMessage:
		v.counter += msg.count
	case *GetCounterMessage:
		ctx.Reply(v.counter)
	}
}

func TestCoordinatorActor(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		system := bootstrap.NewActorSystem(
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
			vivid.WithActorSystemVirtualActorProvider("counter", vivid.ActorProviderFN(func() vivid.Actor {
				return &CounterVirtualActorMock{}
			})),
		)
		assert.NoError(t, system.Start())
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		counterRef := system.VirtualRef("counter", "test-001")

		system.Tell(counterRef, &IncrementMessage{count: 1})
		result, err := system.Ask(counterRef, &GetCounterMessage{}).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), result.(int64))
	})

	t.Run("multiple virtual actors in the same kind", func(t *testing.T) {
		system := bootstrap.NewActorSystem(
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
			vivid.WithActorSystemVirtualActorProvider("counter", vivid.ActorProviderFN(func() vivid.Actor {
				return &CounterVirtualActorMock{}
			})),
		)
		assert.NoError(t, system.Start())
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		for i := 1; i <= 9; i++ {
			name := fmt.Sprintf("test-00%d", i)
			counterRef := system.VirtualRef("counter", name)
			system.Tell(counterRef, &IncrementMessage{count: 1})
			result, err := system.Ask(counterRef, &GetCounterMessage{}).Result()
			assert.NoError(t, err)
			assert.Equal(t, int64(1), result.(int64))
		}
	})
}
