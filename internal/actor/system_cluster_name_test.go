package actor_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystem_ClusterNameMismatch(t *testing.T) {
	address := clusterTestAddress(t)
	seed := startClusterTestSystem(t, address, 0)
	joiner := pendingClusterSystem(t, clusterTestAddress(t),
		vivid.WithActorSystemClusterName("other-cluster"),
		vivid.WithActorSystemClusterSeeds(address),
		vivid.WithActorSystemClusterJoinTimeout(3*time.Second),
	)
	_, err := joiner.ClusterReady(2 * time.Second).Result()
	require.ErrorIs(t, err, vivid.ErrorClusterJoinFailed)
	require.ErrorIs(t, err, vivid.ErrorClusterNameMismatch, "错误种子应明确拒绝，不等待网络超时")
	assert.NotErrorIs(t, err, vivid.ErrorFutureTimeout)
	assert.Contains(t, err.Error(), "other-cluster")
	assert.Contains(t, err.Error(), "vivid-test")
	assert.Contains(t, err.Error(), address)
	for _, system := range []vivid.ActorSystem{seed, joiner} {
		snapshot, err := system.ClusterSnapshot(time.Second).Result()
		require.NoError(t, err)
		assert.Equal(t, 1, snapshot.Len(), "拒绝时双方均不能合并成员")
		local, _ := snapshot.Member(snapshot.LocalID())
		if system == joiner {
			assert.Equal(t, vivid.ClusterMemberStatusJoining, local.Status)
		}
	}
	_, err = joiner.ClusterReady(time.Second).Result()
	assert.ErrorIs(t, err, vivid.ErrorClusterNameMismatch, "重复查询保留本轮初始失败原因")
}

func TestSystem_ClusterNameNextSeed(t *testing.T) {
	addresses := []string{clusterTestAddress(t), clusterTestAddress(t)}
	slices.Sort(addresses)
	wrong := pendingClusterSystem(t, addresses[0], vivid.WithActorSystemClusterName("other-cluster"))
	_, err := wrong.ClusterReady(time.Second).Result()
	require.NoError(t, err)
	seed := startClusterTestSystem(t, addresses[1], 0)
	joiner := pendingClusterSystem(t, clusterTestAddress(t), vivid.WithActorSystemClusterSeeds(addresses...))
	result, err := joiner.ClusterReady(2 * time.Second).Result()
	require.NoError(t, err, "名称不匹配后仍应尝试下一种子")
	assert.Equal(t, addresses[1], result.SeedAddress)
	assert.False(t, result.Bootstrapped)
	for _, system := range []vivid.ActorSystem{seed, joiner} {
		snapshot, err := system.ClusterSnapshot(time.Second).Result()
		require.NoError(t, err)
		assert.Equal(t, 2, snapshot.Len())
	}
	snapshot, err := wrong.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	assert.Equal(t, 1, snapshot.Len(), "错误种子不能从加入或后续周期联络吸收其他组")
}
