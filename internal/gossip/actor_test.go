package gossip_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestActor_Gossip(t *testing.T) {
	system1 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:8080"),
	)

	system2 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:8081"),
	)

	system3 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:8082"),
	)

	assert.NoError(t, system1.Start())
	assert.NoError(t, system2.Start())
	assert.NoError(t, system3.Start())

	defer func() {
		done := make(chan struct{})
		go func() {
			assert.NoError(t, system1.Stop())
			assert.NoError(t, system2.Stop())
			assert.NoError(t, system3.Stop())
			close(done)
		}()

		select {
		case <-time.After(5 * time.Second):
			t.Fatal("timeout")
		case <-done:
		}
	}()

	gossipRef1, err := system1.ActorOf(gossip.New(system1.Logger()))
	assert.NoError(t, err)
	assert.NotNil(t, gossipRef1)

	gossipRef2, err := system2.ActorOf(gossip.New(system2.Logger(), gossip.WithSeeds(gossipRef1.Clone())))
	assert.NoError(t, err)
	assert.NotNil(t, gossipRef2)

	gossipRef3, err := system3.ActorOf(gossip.New(system3.Logger(), gossip.WithSeeds(gossipRef1.Clone())))
	assert.NoError(t, err)
	assert.NotNil(t, gossipRef3)

	time.Sleep(111111 * time.Second)
}

func TestMultiple_Gossip(t *testing.T) {
	var seedNodeCount = 3
	var nodeCount = 10
	var basePort = 8080

	systems := make([]vivid.PrimaryActorSystem, nodeCount)

	for i := 0; i < nodeCount; i++ {
		systems[i] = bootstrap.NewActorSystem(
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
			vivid.WithActorSystemRemoting(fmt.Sprintf("127.0.0.1:%d", basePort+i)),
		)
	}

	for _, system := range systems {
		assert.NoError(t, system.Start())
	}

	defer func() {
		for _, system := range systems {
			assert.NoError(t, system.Stop())
		}
	}()

	var seeds []vivid.ActorRef
	for i := 0; i < nodeCount; i++ {
		system := systems[i]
		gossipActor := gossip.New(system.Logger(), gossip.WithSeeds(seeds...), gossip.WithLaunchDelay(0))
		gossipRef, err := system.ActorOf(gossipActor)
		if assert.NoError(t, err) && assert.NotNil(t, gossipRef) && len(seeds) < seedNodeCount {
			seeds = append(seeds, gossipRef.Clone())
		}
	}

	time.Sleep(time.Hour)
}
