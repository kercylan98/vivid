package actor_test

import (
	"sync"
	"testing"
	"time"

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

func TestSystem_RemotingAsk(t *testing.T) {
	type TestInternalMessage struct {
		Text string `json:"text"`
	}

	codec := NewTestCodec().
		Register("test_message", &TestInternalMessage{})

	system1 := actor.NewSystem(vivid.WithRemoting(codec, "127.0.0.1:8080")).Unwrap()
	system2 := actor.NewSystem(vivid.WithRemoting(codec, "127.0.0.1:8081")).Unwrap()

	ref := system1.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch v := ctx.Message().(type) {
		case *TestInternalMessage:
			ctx.Reply(v)
		}
	})).Unwrap()
	ref = ref.Clone()

	var wg sync.WaitGroup
	wg.Add(1)
	system2.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			f := ctx.Ask(ref, &TestInternalMessage{Text: "hello"}, time.Second*5)
			reply, err := f.Result()
			assert.Nil(t, err)
			m, ok := reply.(*TestInternalMessage)
			assert.True(t, ok)
			assert.True(t, m.Text == "hello")
			wg.Done()
		}
	}))

	wg.Wait()

	system1.Stop()
	system2.Stop()
}
