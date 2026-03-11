package actor_test

import (
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
)

func TestSystem_FindActor(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		system := actor.NewTestSystem(t)

		tempRef, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
		assert.NoError(t, err)
		assert.NotNil(t, tempRef)

		ref, err := system.FindActor(tempRef.String())
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		assert.Equal(t, tempRef.String(), ref.String())
	})

	t.Run("root must not found", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.FindActor(system.Ref().String())
		assert.ErrorIs(t, err, vivid.ErrorNotFound)
		assert.Nil(t, ref)
	})

	t.Run("parse failed", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.FindActor("xxx")
		assert.ErrorIs(t, err, vivid.ErrorRefFormat)
		assert.Nil(t, ref)
	})

	t.Run("not is local", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.FindActor("128.0.0.1:1111/test")
		assert.ErrorIs(t, err, vivid.ErrorNotFound)
		assert.Nil(t, ref)
	})

	t.Run("not found", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.FindActor(actor.LocalAddress + "/test")
		assert.ErrorIs(t, err, vivid.ErrorNotFound)
		assert.Nil(t, ref)
	})
}

func TestSystem_ParseRef(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.ParseRef("example.com:8080/user/worker-1")
		assert.NoError(t, err)
		assert.NotNil(t, ref)
		assert.Equal(t, "example.com:8080", ref.GetAddress())
		assert.Equal(t, "/user/worker-1", ref.GetPath())
	})
	t.Run("parse failed", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		ref, err := system.ParseRef("xxx")
		assert.ErrorIs(t, err, vivid.ErrorRefFormat)
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
	t.Run("stop after", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		assert.NoError(t, system.Stop())
		assert.ErrorIs(t, system.Start(), vivid.ErrorActorSystemAlreadyStopped)
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

	t.Run("not start", func(t *testing.T) {
		system := actor.NewSystem()
		assert.ErrorIs(t, system.Stop(), vivid.ErrorActorSystemNotStarted)
	})
}

func TestSystem_Metrics(t *testing.T) {
	t.Run("metrics", func(t *testing.T) {
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
	})

	t.Run("metrics disabled", func(t *testing.T) {
		system := actor.NewTestSystem(t)
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		snapshot := system.Metrics().Snapshot()
		assert.Equal(t, 0, len(snapshot.Histograms))
		assert.Equal(t, 0, len(snapshot.Gauges))
		assert.Equal(t, 0, len(snapshot.Counters))
	})
}

func TestSystem_WithRemoting(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		system := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8080"))
		defer func() {
			assert.NoError(t, system.Stop())
		}()
	})

	t.Run("no Remoting, send to remote system", func(t *testing.T) {
		normalSystem := actor.NewTestSystem(t)
		remoteSystem := actor.NewTestSystem(t, vivid.WithActorSystemRemoting("127.0.0.1:8080"))
		defer func() {
			assert.NoError(t, normalSystem.Stop())
			assert.NoError(t, remoteSystem.Stop())
		}()

		normalSystem.Tell(remoteSystem.Ref().Clone(), "hello")
	})

	t.Run("send to invalid remote system", func(t *testing.T) {
		system := actor.NewTestSystem(t,
			vivid.WithActorSystemRemoting("127.0.0.1:8080"),
			vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnectLimit(0))) // 不重试
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		wait := make(chan struct{})
		watcher, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.EventStream().Subscribe(ctx, ves.RemotingConnectionFailedEvent{})
			case ves.RemotingConnectionFailedEvent:
				close(wait)
			}
		}))
		assert.NoError(t, err)
		assert.NotNil(t, watcher)

		invalidRemoteRef, err := actor.NewRef("127.0.0.1:8081", "/")
		assert.NoError(t, err)
		assert.NotNil(t, invalidRemoteRef)

		system.Tell(invalidRemoteRef, "hello")

		select {
		case <-wait:
		case <-time.After(time.Second * 3):
			t.Fatal("timeout")
		}
	})
}
