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
	// 该用例创建 3 个种子节点且互相连接，而后立即关闭集群以测试完整生命周期流程

	system1 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:7070"),
	)

	system2 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:8080"),
	)

	system3 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:9090"),
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
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		case <-done:
		}
	}()

	gossipRef1, err := system1.ActorOf(gossip.New(vivid.NewActorSystemClusterOptions()))
	assert.NoError(t, err)
	assert.NotNil(t, gossipRef1)

	gossipRef2, err := system2.ActorOf(gossip.New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterSeeds(gossipRef1.GetAddress()))))
	assert.NoError(t, err)
	assert.NotNil(t, gossipRef2)

	gossipRef3, err := system3.ActorOf(gossip.New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterSeeds(gossipRef1.GetAddress()))))
	assert.NoError(t, err)
	assert.NotNil(t, gossipRef3)
}
