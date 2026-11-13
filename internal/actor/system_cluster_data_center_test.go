package actor_test

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystem_ClusterDataCenter(t *testing.T) {
	address := clusterTestAddress(t)
	seed := pendingClusterSystem(t, address, vivid.WithActorSystemClusterDataCenter("dc-a"))
	_, err := seed.ClusterReady(time.Second).Result()
	require.NoError(t, err)
	before, err := seed.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	joiner := pendingClusterSystem(t, clusterTestAddress(t), vivid.WithActorSystemClusterDataCenter("dc-b"), vivid.WithActorSystemClusterSeeds(address))
	result, err := joiner.ClusterReady(2 * time.Second).Result()
	require.NoError(t, err, "不同机房的同名节点应正常加入")
	assert.Equal(t, address, result.SeedAddress)
	joined, err := joiner.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	local, found := joined.Member(joined.LocalID())
	require.True(t, found)
	assert.Equal(t, "dc-b", local.DataCenter)
	remote, found := joined.Member(before.LocalID())
	require.True(t, found)
	assert.Equal(t, "dc-a", remote.DataCenter)
	require.Eventually(t, func() bool {
		snapshot, err := seed.ClusterSnapshot(time.Second).Result()
		if err != nil {
			return false
		}
		member, found := snapshot.Member(joined.LocalID())
		return found && member.Status == vivid.ClusterMemberStatusUp && member.DataCenter == "dc-b"
	}, 2*time.Second, 20*time.Millisecond)
	assert.Equal(t, 1, before.Len(), "加入不改变旧快照")
	unknown := pendingClusterSystem(t, clusterTestAddress(t), vivid.WithActorSystemClusterSeeds(address))
	_, err = unknown.ClusterReady(2 * time.Second).Result()
	require.NoError(t, err, "未声明机房也可加入")
	snapshot, err := unknown.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	member, _ := snapshot.Member(snapshot.LocalID())
	assert.Empty(t, member.DataCenter)
}
