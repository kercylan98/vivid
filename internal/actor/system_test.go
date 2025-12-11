package actor_test

import (
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/stretchr/testify/assert"
)

func TestSystem_Stop(t *testing.T) {
	system := actor.NewSystem().Unwrap()

	var wg sync.WaitGroup
	wg.Add(3)
	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			wg.Done()
		case *vivid.OnKill, *vivid.OnKilled:
			wg.Done()
		}
	}))

	system.Stop()
	wg.Wait()
}

// TODO: 当前跨网络暂无法发出，待修复
func TestSystem_RemotingAsk(t *testing.T) {
	system1 := actor.NewSystem(vivid.WithRemoting("127.0.0.1:8080")).Unwrap()
	system2 := actor.NewSystem(vivid.WithRemoting("127.0.0.1:8081")).Unwrap()

	ref := system1.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			if ctx.Sender() != nil {
				ctx.Reply(&vivid.OnLaunch{})
				return
			}
		}
	})).Unwrap()
	ref = ref.Clone()

	var wg sync.WaitGroup
	wg.Add(1)
	system2.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			f := ctx.Ask(ref, &vivid.OnLaunch{})
			reply, err := f.Result()
			assert.Nil(t, err)
			_, ok := reply.(*vivid.OnLaunch)
			assert.True(t, ok)
			wg.Done()
		}
	}))

	wg.Wait()

	system1.Stop()
	system2.Stop()
}
