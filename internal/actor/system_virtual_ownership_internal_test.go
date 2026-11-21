package actor

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/stretchr/testify/require"
)

func (n *raftRuntimeNetwork) ownership(t *testing.T, id uint64, kind, name string) (clusterstore.VirtualOwnership, bool) {
	t.Helper()
	key, err := clusterstore.VirtualOwnershipKey(kind, name)
	require.NoError(t, err)
	ownership, exists, err := clusterstore.ReadVirtualOwnership(kind, name, n.consistentGet(t, id, key))
	require.NoError(t, err)
	return ownership, exists
}

func (n *raftRuntimeNetwork) changeOwnership(t *testing.T, id uint64, change clusterstore.VirtualOwnershipChange) (clusterstore.VirtualOwnership, clusterstore.RaftRuntimeExecution, error) {
	t.Helper()
	result, err := n.systems[id].Ask(n.refs[id], change.Request(0), 6*time.Second).Result()
	require.NoError(t, err)
	execution := result.(clusterstore.RaftRuntimeExecution)
	ownership, err := change.Result(execution)
	return ownership, execution, err
}

func TestSystem_VirtualOwnershipCompetitionAndPartition(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	var claims []clusterstore.VirtualOwnershipChange
	var futures []vivid.Future[vivid.Message]
	for id := uint64(1); id <= 2; id++ {
		owner, err := clusterstore.NewVirtualOwner(n.refs[id].GetAddress())
		require.NoError(t, err)
		claim, err := clusterstore.ClaimVirtualOwnership("counter", "one", owner, 0)
		require.NoError(t, err)
		claims = append(claims, claim)
		// 控制请求进入 Leader 本地入口，三个 Raft 副本之间使用真实 TCP 复制。
		futures = append(futures, n.systems[leader].Ask(n.refs[leader], claim.Request(0), 6*time.Second))
	}
	var claimed clusterstore.VirtualOwnership
	winners := 0
	for i, future := range futures {
		result, err := future.Result()
		require.NoError(t, err)
		got, err := claims[i].Result(result.(clusterstore.RaftRuntimeExecution))
		if err == nil {
			claimed = got
			winners++
		} else {
			require.ErrorIs(t, err, clusterstore.ErrVirtualOwnershipConflict)
			require.Zero(t, got)
		}
	}
	require.Equal(t, 1, winners, "相同逻辑身份并发争抢只允许一个取得归属")
	current, exists := n.ownership(t, leader, "counter", "one")
	require.True(t, exists)
	require.Equal(t, claimed, current)
	release, err := claimed.Release()
	require.NoError(t, err)
	n.committed(t, n.status(t, leader).Commit, 1, 2, 3)
	n.partition(leader)
	result, err := n.systems[leader].Ask(n.refs[leader], release.Request(80*time.Millisecond), 2*time.Second).Result()
	require.NoError(t, err)
	got, err := release.Result(result.(clusterstore.RaftRuntimeExecution))
	require.ErrorIs(t, err, clusterstore.ErrRaftOutcomeUnknown)
	require.Zero(t, got, "少数派的本地接纳不得宣告释放成功")
	var majority []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != leader {
			majority = append(majority, id)
		}
	}
	nextLeader := n.leader(t, majority...)
	current, exists = n.ownership(t, nextLeader, "counter", "one")
	require.True(t, exists)
	require.Equal(t, claimed, current, "少数派未提交的释放不得进入多数派归属")
	nextOwner, err := clusterstore.NewVirtualOwner(claimed.Owner.Address)
	require.NoError(t, err)
	require.NotEqual(t, claimed.Owner.InstanceID, nextOwner.InstanceID)
	transfer, err := clusterstore.ClaimVirtualOwnership("counter", "one", nextOwner, claimed.Generation)
	require.NoError(t, err)
	transferred, execution, err := n.changeOwnership(t, nextLeader, transfer)
	require.NoError(t, err)
	require.Greater(t, transferred.Generation, claimed.Generation)
	require.Equal(t, execution.Index, transferred.Generation)
	n.partition()
	n.committed(t, execution.Index, 1, 2, 3)
	_, rejected, err := n.changeOwnership(t, nextLeader, release)
	require.ErrorIs(t, err, clusterstore.ErrVirtualOwnershipConflict, "旧释放在恢复后不能删除新代次")
	n.committed(t, rejected.Index, 1, 2, 3)
	current, exists = n.ownership(t, nextLeader, "counter", "one")
	require.True(t, exists)
	require.Equal(t, transferred, current)
}

func TestSystem_VirtualOwnershipSnapshotAndWALRecovery(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	owner, err := clusterstore.NewVirtualOwner(n.refs[leader].GetAddress())
	require.NoError(t, err)
	claim, err := clusterstore.ClaimVirtualOwnership("counter", "订单/a", owner, 0)
	require.NoError(t, err)
	claimed, _, err := n.changeOwnership(t, leader, claim)
	require.NoError(t, err)
	// 第二个名字独立保留归属，用于同时核对快照中的存在与缺失记录。
	other, err := clusterstore.ClaimVirtualOwnership("counter", "two", owner, 0)
	require.NoError(t, err)
	kept, _, err := n.changeOwnership(t, leader, other)
	require.NoError(t, err)
	release, err := claimed.Release()
	require.NoError(t, err)
	_, released, err := n.changeOwnership(t, leader, release)
	require.NoError(t, err)
	n.committed(t, released.Index, 1, 2, 3)
	for id := uint64(1); id <= 3; id++ {
		result, err := n.systems[id].Ask(n.refs[id], clusterstore.RaftRuntimeCompact{}, time.Second).Result()
		require.NoError(t, err)
		require.NoError(t, result.(clusterstore.RaftRuntimeResult).Err)
	}
	for id := uint64(1); id <= 3; id++ {
		require.NoError(t, n.systems[id].Stop(3*time.Second))
	}
	for id := uint64(1); id <= 3; id++ {
		n.start(t, id, false)
	}
	leader = n.leader(t, 1, 2, 3)
	_, exists := n.ownership(t, leader, claimed.Kind, claimed.Name)
	require.False(t, exists)
	current, exists := n.ownership(t, leader, kept.Kind, kept.Name)
	require.True(t, exists)
	require.Equal(t, kept, current, "共识节点重启不改变已有所有者的启动身份与代次")
	restarted, err := clusterstore.NewVirtualOwner(owner.Address)
	require.NoError(t, err)
	newClaim, err := clusterstore.ClaimVirtualOwnership(claimed.Kind, claimed.Name, restarted, 0)
	require.NoError(t, err)
	reacquired, execution, err := n.changeOwnership(t, leader, newClaim)
	require.NoError(t, err)
	require.Greater(t, reacquired.Generation, released.Index, "快照恢复后的新代次须超过释放前的整个日志前缀")
	require.Equal(t, restarted, reacquired.Owner)
	_, _, err = n.changeOwnership(t, leader, release)
	require.ErrorIs(t, err, clusterstore.ErrVirtualOwnershipConflict)
	current, exists = n.ownership(t, leader, reacquired.Kind, reacquired.Name)
	require.True(t, exists)
	require.Equal(t, reacquired, current)
	n.committed(t, execution.Index, 1, 2, 3)
}
