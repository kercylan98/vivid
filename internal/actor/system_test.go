package actor_test

import (
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/stretchr/testify/assert"
)

func TestSystem_FindActorRef(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		system := actor.NewTestSystem(t)

		tempRef, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
		assert.NoError(t, err)
		assert.NotNil(t, tempRef)

		ref, err := system.FindActorRef(tempRef.String())
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		assert.Equal(t, tempRef.String(), ref.String())
	})

	t.Run("root must not found", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.FindActorRef(system.Ref().String())
		assert.ErrorIs(t, err, vivid.ErrorNotFound)
		assert.Nil(t, ref)
	})

	t.Run("parse failed", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.FindActorRef("xxx")
		assert.ErrorIs(t, err, vivid.ErrorRefFormat)
		assert.Nil(t, ref)
	})

	t.Run("not is local", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.FindActorRef("128.0.0.1:1111/test")
		assert.ErrorIs(t, err, vivid.ErrorNotFound)
		assert.Nil(t, ref)
	})

	t.Run("not found", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.FindActorRef(actor.LocalAddress + "/test")
		assert.ErrorIs(t, err, vivid.ErrorNotFound)
		assert.Nil(t, ref)
	})

}

func TestSystem_New(t *testing.T) {
	system := actor.NewSystem()
	assert.NotNil(t, system)
}

func TestSystem_ActorOfWithConcurrency(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case *vivid.OnLaunch:
					wg.Done()
				}
			}))

			assert.NotNil(t, ref)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
}

func TestSystem_Start(t *testing.T) {

	t.Run("repeated start", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		assert.ErrorIs(t, system.Start(), vivid.ErrorActorSystemAlreadyStarted)
	})

	t.Run("start failed", func(t *testing.T) {
		system := actor.NewSystem(vivid.WithActorSystemRemoting("127.0.0.1"))
		assert.ErrorIs(t, system.Start(), vivid.ErrorActorSystemStartFailed)
	})
}

func TestSystem_Stop(t *testing.T) {
	t.Run("normal stop", func(t *testing.T) {
		system := actor.NewTestSystem(t)

		var wg sync.WaitGroup
		wg.Add(3)
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				wg.Done()
			case *vivid.OnKill, *vivid.OnKilled:
				wg.Done()
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		assert.NoError(t, system.Stop())
		wg.Wait()
	})

	t.Run("timeout", func(t *testing.T) {
		system := actor.NewTestSystem(t)

		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			time.Sleep(time.Millisecond * 100)
		}))

		assert.NotNil(t, ref)
		assert.NoError(t, err)

		assert.ErrorIs(t, system.Stop(time.Millisecond), vivid.ErrorActorSystemStopFailed)
	})

	t.Run("repeat stop", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		assert.NoError(t, system.Stop())
		assert.ErrorIs(t, system.Stop(), vivid.ErrorActorSystemAlreadyStopped)
	})
}

func TestSystem_RemotingAsk(t *testing.T) {
	type TestInternalMessage struct {
		Text string `json:"text"`
	}

	codec := NewTestCodec().
		Register("test_message", &TestInternalMessage{})

	system1 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8080"), vivid.WithActorSystemCodec(codec))
	system2 := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8081"), vivid.WithActorSystemCodec(codec), vivid.WithActorSystemLogger(log.GetDefault()))

	ref, err := system1.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch v := ctx.Message().(type) {
		case *TestInternalMessage:
			ctx.Reply(v)
		}
	}))
	assert.NotNil(t, ref)
	assert.NoError(t, err)
	ref = ref.Clone()

	var wg sync.WaitGroup
	wg.Add(1)
	_, err = system2.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
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
	assert.NoError(t, err)

	wg.Wait()

	assert.NoError(t, system1.Stop())
	assert.NoError(t, system2.Stop())
}

func TestSystem_Metrics(t *testing.T) {
	system := actor.NewTestSystem(t, vivid.WithActorSystemEnableMetrics(true))
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.Metrics().Counter("test_counter").Inc()
			ctx.Metrics().Gauge("test_gauge").Inc()
			ctx.Metrics().Histogram("test_histogram").Observe(1)
			wg.Done()
		}
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ref)

	wg.Wait()
	snapshot := system.Metrics().Snapshot()
	assert.Equal(t, uint64(1), snapshot.Counters["test_counter"])
	assert.Equal(t, int64(1), snapshot.Gauges["test_gauge"])
	assert.Equal(t, metrics.HistogramSnapshot{Count: 0x1, Sum: 1, Min: 1, Max: 1, Values: []float64{1}}, snapshot.Histograms["test_histogram"])
}

func TestSystem_HandleRemotingEnvelop_InvalidAgentRef(t *testing.T) {
	system := actor.NewTestSystem(t)
	defer func() {
		assert.Nil(t, recover())
		assert.NoError(t, system.Stop())
	}()

	// 获取系统根引用的合法地址和路径
	rootRef := system.Ref()

	agentAddr := rootRef.GetAddress()
	agentPath := rootRef.GetPath()
	senderAddr := rootRef.GetAddress()
	senderPath := rootRef.GetPath()
	receiverAddr := rootRef.GetAddress()
	receiverPath := rootRef.GetPath()

	// 构造非法的 agent 地址：不带端口的裸 IP，会被 NormalizeAddress 拒绝
	invalidAddr := "127.0.0.1"

	err := system.HandleRemotingEnvelop(false, invalidAddr, agentPath, senderAddr, senderPath, receiverAddr, receiverPath, "test message")
	assert.NotNil(t, err)
	err = system.HandleRemotingEnvelop(false, agentAddr, agentPath, invalidAddr, senderPath, receiverAddr, receiverPath, "test message")
	assert.NotNil(t, err)
	err = system.HandleRemotingEnvelop(false, agentAddr, agentPath, senderAddr, senderPath, invalidAddr, receiverPath, "test message")
	assert.NotNil(t, err)
}
