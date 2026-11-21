package actor_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystem_ClusterShutdownSampling(t *testing.T) {
	for _, forced := range []bool{false, true} {
		name, interval, budget := "sampled", 20*time.Millisecond, 2*time.Second
		if forced {
			name, interval, budget = "budget", time.Hour, 80*time.Millisecond
		}
		t.Run(name, func(t *testing.T) {
			system := pendingClusterSystem(t, clusterTestAddress(t),
				vivid.WithActorSystemClusterGossipInterval(interval),
				vivid.WithActorSystemClusterGracefulShutdownTimeout(budget),
			)
			_, err := system.ClusterReady(time.Second).Result()
			require.NoError(t, err)
			started := time.Now()
			require.NoError(t, system.Stop(3*time.Second))
			elapsed := time.Since(started)
			if forced {
				assert.GreaterOrEqual(t, elapsed, budget, "周期长于退出预算时仍由原预算结束等待")
			} else {
				assert.GreaterOrEqual(t, elapsed, 9*interval, "三个退出阶段分别完成周期采样")
				assert.Less(t, elapsed, budget, "周期采样应主动完成，不能仅等待强制退出")
			}
		})
	}
}

func TestSystem_ClusterStopWaitTimeout(t *testing.T) {
	seed := pendingClusterSystem(t, clusterTestAddress(t),
		vivid.WithActorSystemClusterDataCenter("dc-a"),
		vivid.WithActorSystemClusterGossipInterval(20*time.Millisecond),
	)
	_, err := seed.ClusterReady(time.Second).Result()
	require.NoError(t, err)
	address := clusterTestAddress(t)
	joining := pendingClusterSystem(t, address,
		vivid.WithActorSystemClusterDataCenter("dc-b"),
		vivid.WithActorSystemClusterSeeds(seed.Ref().GetAddress()),
		vivid.WithActorSystemClusterGossipInterval(20*time.Millisecond),
	)
	joined, err := joining.ClusterReady(time.Second).Result()
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		snapshot, err := seed.ClusterSnapshot(time.Second).Result()
		member, exists := snapshot.Member(joined.LocalID)
		return err == nil && exists && member.Status == vivid.ClusterMemberStatusUp
	}, time.Second, 5*time.Millisecond)

	err = joining.Stop(20 * time.Millisecond)
	require.ErrorIs(t, err, vivid.ErrorActorSystemStopFailed)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	// 第一个等待者超时后，周期采样与远程链路仍须支持后续有序退出。
	require.NoError(t, joining.Stop(2*time.Second))
	require.Eventually(t, func() bool {
		snapshot, err := seed.ClusterSnapshot(time.Second).Result()
		member, exists := snapshot.Member(joined.LocalID)
		return err == nil && exists && member.Status == vivid.ClusterMemberStatusRemoved
	}, time.Second, 5*time.Millisecond, "必须先传播退出状态，再关闭 Remoting")
	listener, err := net.Listen("tcp", address)
	require.NoError(t, err)
	require.NoError(t, listener.Close())
}
