package gossip_test

import (
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
		assert.NoError(t, system1.Stop())
		assert.NoError(t, system2.Stop())
		assert.NoError(t, system3.Stop())
	}()

	gossipRef1, err := system1.ActorOf(gossip.New())
	assert.NoError(t, err)
	assert.NotNil(t, gossipRef1)

	gossipRef2, err := system2.ActorOf(gossip.New(gossipRef1.Clone()))
	assert.NoError(t, err)
	assert.NotNil(t, gossipRef2)

	gossipRef3, err := system3.ActorOf(gossip.New(gossipRef1.Clone()))
	assert.NoError(t, err)
	assert.NotNil(t, gossipRef3)

	time.Sleep(10 * time.Second)
}
