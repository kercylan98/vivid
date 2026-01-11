package actor_test

import (
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
)

func TestEventStream_Subscribe(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()
	var waitSub = make(chan struct{})
	var waitEvent = make(chan struct{})
	var once sync.Once
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.EventStream().Subscribe(ctx, ves.ActorSpawnedEvent{})
			close(waitSub)
		case vivid.StreamEvent:
			once.Do(func() {
				close(waitEvent)
			})
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	<-waitSub
	ref, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	<-waitEvent
}
