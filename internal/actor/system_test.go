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

func TestSystem_Stop(t *testing.T) {
	system := actor.NewTestSystem(t)

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

	system1 := actor.NewTestSystem(t, vivid.WithRemoting(codec, "127.0.0.1:8080"))
	system2 := actor.NewTestSystem(t, vivid.WithRemoting(codec, "127.0.0.1:8081"), vivid.WithActorSystemLogger(log.GetDefault()))

	ref, err := system1.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch v := ctx.Message().(type) {
		case *TestInternalMessage:
			ctx.Reply(v)
		}
	}))
	assert.NoError(t, err)
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

func TestSystem_Metrics(t *testing.T) {
	system := actor.NewTestSystem(t, vivid.WithActorSystemEnableMetrics(true))
	defer system.Stop()

	wg := sync.WaitGroup{}
	wg.Add(1)
	system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.Metrics().Counter("test_counter").Inc()
			ctx.Metrics().Gauge("test_gauge").Inc()
			ctx.Metrics().Histogram("test_histogram").Observe(1)
			wg.Done()
		}
	}))

	wg.Wait()
	snapshot := system.Metrics().Snapshot()
	assert.Equal(t, uint64(1), snapshot.Counters["test_counter"])
	assert.Equal(t, int64(1), snapshot.Gauges["test_gauge"])
	assert.Equal(t, metrics.HistogramSnapshot{Count: 0x1, Sum: 1, Min: 1, Max: 1, Values: []float64{1}}, snapshot.Histograms["test_histogram"])
}
