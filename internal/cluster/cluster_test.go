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

func TestCluster_DataCenter(t *testing.T) {
	asiaChina1 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:8080"),
		vivid.WithActorSystemRemotingOptions(
			vivid.NewActorSystemRemotingOptions(),
			vivid.WithActorSystemRemotingClusterOption(
				vivid.WithClusterName("MyCluster"),
				vivid.WithClusterDatacenter("AsiaChina"),
				vivid.WithClusterRegion("Asia"),
				vivid.WithClusterRack("AsiaChina1"),
				vivid.WithClusterSeedsByDC(map[string][]string{
					"AsiaChina":    {"127.0.0.1:8080"},
					"NorthAmerica": {"127.0.0.1:8082"},
				}),
			),
		),
	)

	asiaChina2 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:8081"),
		vivid.WithActorSystemRemotingOptions(
			vivid.NewActorSystemRemotingOptions(),
			vivid.WithActorSystemRemotingClusterOption(
				vivid.WithClusterName("MyCluster"),
				vivid.WithClusterDatacenter("AsiaChina"),
				vivid.WithClusterRegion("Asia"),
				vivid.WithClusterRack("AsiaChina1"),
				vivid.WithClusterSeedsByDC(map[string][]string{
					"AsiaChina":    {"127.0.0.1:8080"},
					"NorthAmerica": {"127.0.0.1:8082"},
				}),
			),
		),
	)

	northAmerica1 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:8082"),
		vivid.WithActorSystemRemotingOptions(
			vivid.NewActorSystemRemotingOptions(),
			vivid.WithActorSystemRemotingClusterOption(
				vivid.WithClusterName("MyCluster"),
				vivid.WithClusterDatacenter("NorthAmerica"),
				vivid.WithClusterRegion("NorthAmerica"),
				vivid.WithClusterRack("NorthAmerica1"),
				vivid.WithClusterSeedsByDC(map[string][]string{
					"NorthAmerica": {"127.0.0.1:8082"},
					"AsiaChina":    {"127.0.0.1:8080"},
				}),
			),
		),
	)

	northAmerica2 := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
		vivid.WithActorSystemRemoting("127.0.0.1:8083"),
		vivid.WithActorSystemRemotingOptions(
			vivid.NewActorSystemRemotingOptions(),
			vivid.WithActorSystemRemotingClusterOption(
				vivid.WithClusterName("MyCluster"),
				vivid.WithClusterDatacenter("NorthAmerica"),
				vivid.WithClusterRegion("NorthAmerica"),
				vivid.WithClusterRack("NorthAmerica2"),
				vivid.WithClusterSeedsByDC(map[string][]string{
					"NorthAmerica": {"127.0.0.1:8082"},
					"AsiaChina":    {"127.0.0.1:8080"},
				}),
			),
		),
	)

	assert.NoError(t, asiaChina1.Start())
	assert.NoError(t, asiaChina2.Start())
	assert.NoError(t, northAmerica2.Start())
	assert.NoError(t, northAmerica1.Start())
	defer func() {
		assert.NoError(t, asiaChina1.Stop())
		assert.NoError(t, asiaChina2.Stop())
		assert.NoError(t, northAmerica1.Stop())
		assert.NoError(t, northAmerica2.Stop())
	}()

	wait := make(chan struct{})

	time.AfterFunc(time.Second*3, func() {
		close(wait)
	})
	for {
		select {
		case <-time.After(time.Millisecond * 100):
			members, err := northAmerica1.Cluster().GetMembers()
			if !assert.NoError(t, err) {
				return
			}
			if len(members) == 4 {
				asiaChina1.Logger().Info("cluster members", log.Any("members", members))
				return
			}
		case <-wait:
			assert.Fail(t, "timeout")
			return
		}
	}
}
