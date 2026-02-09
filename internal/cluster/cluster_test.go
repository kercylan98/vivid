package cluster_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
)

func TestCluster_SingleNode(t *testing.T) {
	system := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:8080"),
		vivid.WithActorSystemRemotingOptions(
			vivid.NewActorSystemRemotingOptions(),
			vivid.WithActorSystemRemotingClusterOption(),
		),
	)

	assert.NoError(t, system.Start())
	defer func() {
		assert.NoError(t, system.Stop())
	}()

	cluster := system.Cluster()
	assert.NotNil(t, cluster)

	members, err := cluster.GetMembers()
	assert.NoError(t, err)
	assert.Len(t, members, 1)
}

func TestCluster_MultiNode(t *testing.T) {
	const nodeCount = 3
	const basePort = 8080
	nodes := make([]vivid.ActorSystem, nodeCount)
	seeds := make([]string, nodeCount)
	defer func() {
		for _, system := range nodes {
			assert.NoError(t, system.Stop())
		}
	}()

	wait := make(chan struct{})

	for i := 0; i < nodeCount; i++ {
		addr := fmt.Sprintf("127.0.0.1:%d", basePort+i)
		if i == 0 {
			seeds = append(seeds, addr)
		}
		system := bootstrap.NewActorSystem(
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
			vivid.WithActorSystemRemoting(addr),
			vivid.WithActorSystemRemotingOptions(
				vivid.NewActorSystemRemotingOptions(),
				vivid.WithActorSystemRemotingClusterOption(
					vivid.WithClusterSeeds(seeds),
				),
			),
		)
		assert.NoError(t, system.Start())
		nodes[i] = system

		if i == 0 {
			watcherRef, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch m := ctx.Message().(type) {
				case *vivid.OnLaunch:
					ctx.EventStream().Subscribe(ctx, ves.ClusterMembersChangedEvent{})
				case ves.ClusterMembersChangedEvent:
					// 是否所有节点都加入了集群
					if len(m.Members) == nodeCount {
						close(wait)
					}
				}
			}))
			assert.NoError(t, err)
			assert.NotNil(t, watcherRef)
		}
	}

	select {
	case <-wait:
	case <-time.After(3 * time.Second):
		assert.Fail(t, "timeout")
		return
	}

}
