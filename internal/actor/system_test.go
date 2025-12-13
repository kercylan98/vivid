package actor_test

import (
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/stretchr/testify/assert"
)

type TestInternalMessage struct {
	text string
}

func init() {
	messages.RegisterInternalMessage[*TestInternalMessage]("test_message",
		func(message any, reader *messages.Reader) error {
			m := message.(*TestInternalMessage)
			return reader.ReadInto(&m.text)
		},
		func(message any, writer *messages.Writer) error {
			m := message.(*TestInternalMessage)
			return writer.WriteFrom(m.text)
		})
}

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

// TODO: Stop 时候 accept Actor无法处理 OnKill
func TestSystem_RemotingAsk(t *testing.T) {
	system1 := actor.NewSystem(vivid.WithRemoting("127.0.0.1:8080")).Unwrap()
	system2 := actor.NewSystem(vivid.WithRemoting("127.0.0.1:8081")).Unwrap()

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
			f := ctx.Ask(ref, &TestInternalMessage{text: "hello"}, time.Second*5)
			reply, err := f.Result()
			assert.Nil(t, err)
			m, ok := reply.(*TestInternalMessage)
			assert.True(t, ok)
			assert.True(t, m.text == "hello")
			wg.Done()
		}
	}))

	wg.Wait()

	system1.Stop()
	system2.Stop()
}
