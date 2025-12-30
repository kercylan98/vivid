package actor_test

import (
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/ves"
)

func TestEventStream_Subscribe(t *testing.T) {
	system := actor.NewTestSystem(t)
	var waitSub = make(chan struct{})
	var waitEvent = make(chan struct{})
	var once sync.Once
	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
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

	<-waitSub
	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))

	<-waitEvent
	system.Stop()
}
