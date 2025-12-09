package actor_test

import (
	"sync"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/stretchr/testify/assert"
)

func TestContext_ActorOf(t *testing.T) {
	system := actor.NewSystem()

	var wg sync.WaitGroup
	wg.Add(1)
	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			wg.Done()
		}
	}))

	wg.Wait()
}

func TestContext_Become(t *testing.T) {
	system := actor.NewSystem()

	var wg sync.WaitGroup
	wg.Add(1)
	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
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

	wg.Wait()
}

func TestContext_RevertBehavior(t *testing.T) {
	system := actor.NewSystem()

	var wg sync.WaitGroup
	wg.Add(2)
	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.Become(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case int:
					wg.Done()
					assert.True(t, ctx.RevertBehavior())
					ctx.Tell(ctx.Ref(), 2)
				}
			})
			ctx.Tell(ctx.Ref(), 1)
		case int:
			// 在 revert 后收到第二次消息，behavior 已复原，这里再次还原应该返回 false
			assert.False(t, ctx.RevertBehavior())
			wg.Done()
		}
	}))

	wg.Wait()
}

func TestContext_Tell(t *testing.T) {
	system := actor.NewSystem()

	var wg sync.WaitGroup
	wg.Add(1)
	ref := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case int:
			wg.Done()
		}
	}))

	system.Tell(ref, 1)
	wg.Wait()
}

func BenchmarkContext_Tell(b *testing.B) {
	system := actor.NewSystem()
	ref := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Tell(ref, 1)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func TestContext_Ask(t *testing.T) {
	system := actor.NewSystem()

	var wg sync.WaitGroup
	wg.Add(3)
	ref := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case int:
			ctx.Reply(1)
			wg.Done()
		}
	}))

	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			reply, err := system.Ask(ref, 1).Result()
			assert.Nil(t, err)
			assert.Equal(t, 1, reply.(int))
			wg.Done()
		}
	}))

	systemReply, systemErr := system.Ask(ref, 1).Result()
	assert.Nil(t, systemErr)
	assert.Equal(t, 1, systemReply.(int))

	wg.Wait()
}

func BenchmarkContext_Ask(b *testing.B) {
	system := actor.NewSystem()
	ref := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case int:
			ctx.Reply(message)
		}
	}))
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
	system := actor.NewSystem()
	var refAHeldRefB vivid.ActorRef
	var wg sync.WaitGroup
	wg.Add(1)

	refA := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch m := ctx.Message().(type) {
		case vivid.ActorRef:
			refAHeldRefB = m
		case string:
			assert.Equal(t, refAHeldRefB.GetAddress().String()+refAHeldRefB.GetPath(), ctx.Sender().GetAddress().String()+ctx.Sender().GetPath(), "sender ref mismatch")
			wg.Done()
		}
	}))

	refB := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case string:
			ctx.Ask(refA, "ask-ref-b")
		}
	}))

	system.Tell(refA, refB)
	system.Tell(refB, "start-ask")
	wg.Wait()
}
