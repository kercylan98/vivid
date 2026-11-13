package gossip_test

import (
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_GossipClusterNameDiscovery(t *testing.T) {
	for _, otherName := range []string{"team-a", "team-b"} {
		t.Run(otherName, func(t *testing.T) {
			var systems []vivid.PrimaryActorSystem
			var addresses []string
			for range 2 {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				address := listener.Addr().String()
				require.NoError(t, listener.Close())
				system := bootstrap.NewActorSystem(
					vivid.WithActorSystemRemoting(address),
					vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
				)
				require.NoError(t, system.Start())
				t.Cleanup(func() { require.NoError(t, system.Stop(3*time.Second)) })
				systems = append(systems, system)
				addresses = append(addresses, address)
			}
			var refs []vivid.ActorRef
			for index, name := range []string{"team-a", otherName} {
				member := gossip.NewSnapshotActor(vivid.NewActorSystemClusterOptions(
					vivid.WithActorSystemClusterName(name),
					vivid.WithActorSystemClusterSeeds(addresses[1-index]),
					vivid.WithActorSystemClusterBootstrap(true),
					vivid.WithActorSystemClusterLaunchDelay(0),
					vivid.WithActorSystemClusterGossipInterval(30*time.Millisecond),
					vivid.WithActorSystemClusterGracefulShutdownTimeout(time.Second),
				))
				ref, err := systems[index].ActorOf(member)
				require.NoError(t, err)
				refs = append(refs, ref)
				result, err := member.Readiness().Wait(systems[index], time.Second).Result()
				require.NoError(t, err)
				assert.True(t, result.Bootstrapped)
			}
			var snapshots [2]gossip.SnapshotResponse
			// 在 Actor 线程采样；确认双向真实收到多轮 Ping 后再判断隔离，避免把未建连当成拒绝成功。
			require.Eventually(t, func() bool {
				for index := range systems {
					message, err := systems[index].Ask(refs[index], gossip.TestSnapshotRequest{}, time.Second).Result()
					if err != nil {
						return false
					}
					snapshots[index] = message.(gossip.SnapshotResponse)
					if snapshots[index].PingCount[addresses[1-index]] < 3 {
						return false
					}
				}
				return true
			}, 3*time.Second, 20*time.Millisecond)
			for index, snapshot := range snapshots {
				assert.Equal(t, endpoint.StatusUp, snapshot.Status)
				if otherName == "team-a" {
					assert.Len(t, snapshot.View.Members().List(), 2, "同组自举节点应从未知种子互相发现")
				} else {
					assert.Len(t, snapshot.View.Members().List(), 1)
					assert.Nil(t, snapshot.View.Members().Get(snapshots[1-index].ID))
					assert.Zero(t, snapshot.View.Version().Get(snapshots[1-index].ID))
				}
			}
		})
	}
}
