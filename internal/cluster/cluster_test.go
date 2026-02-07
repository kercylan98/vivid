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

func TestCluster(t *testing.T) {
	t.Run("single node", func(t *testing.T) {
		system := bootstrap.NewActorSystem(
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
			vivid.WithActorSystemRemoting("127.0.0.1:8080"),
			vivid.WithActorSystemRemotingOption(
				vivid.WithActorSystemRemotingClusterOption(),
			),
		)

		assert.NoError(t, system.Start())
		defer func() {
			assert.NoError(t, system.Stop())
		}()

		clusterContext := system.Cluster()
		members, err := clusterContext.GetMembers()
		assert.NoError(t, err)
		assert.Len(t, members, 1)
	})

	t.Run("multiple nodes", func(t *testing.T) {
		const nodeCount = 3
		const basePort = 8090 // 与 single_node 的 8080 错开，避免端口冲突

		assert.Greater(t, nodeCount, 1)
		assert.Greater(t, basePort, 0)
		assert.Greater(t, basePort+nodeCount-1, 0)

		var systems = make([]vivid.ActorSystem, nodeCount)
		var seed = make([]string, 0)
		var wait = make(chan struct{})
		for i := 0; i < nodeCount; i++ {
			bindAddr := fmt.Sprintf("127.0.0.1:%d", basePort+i)
			system := bootstrap.NewActorSystem(
				vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))),
				vivid.WithActorSystemRemoting(bindAddr),
				vivid.WithActorSystemRemotingOption(
					vivid.WithActorSystemRemotingClusterOption(
						vivid.WithClusterSeeds(seed),
						vivid.WithClusterDiscoveryInterval(500*time.Millisecond),
					),
				),
			)
			assert.NoError(t, system.Start())
			systems[i] = system

			if i == 0 {
				seed = append(seed, bindAddr)
				watcher, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch m := ctx.Message().(type) {
					case *vivid.OnLaunch:
						ctx.EventStream().Subscribe(ctx, ves.ClusterMembersChangedEvent{})
					case ves.ClusterMembersChangedEvent:
						ctx.Logger().Info("cluster members changed", log.Any("members", m))
						if len(m.Members) == nodeCount {
							ctx.Logger().Info("all nodes joined")
							close(wait)
						}
					}
				}))
				assert.NoError(t, err)
				assert.NotNil(t, watcher)
			}
		}

		defer func() {
			for _, system := range systems {
				assert.NoError(t, system.Stop())
			}
		}()

		select {
		case <-wait:
		case <-time.After(time.Second * 3):
			assert.Fail(t, "timeout")
		}
	})
}
