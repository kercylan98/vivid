package actor

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/stretchr/testify/require"
)

func (n *raftRuntimeNetwork) consistentGet(t *testing.T, id uint64, key string) clusterstore.RaftRuntimeRecord {
	t.Helper()
	result, err := n.systems[id].Ask(n.refs[id], clusterstore.RaftRuntimeGet{Key: key}, 6*time.Second).Result()
	require.NoError(t, err)
	return result.(clusterstore.RaftRuntimeRecord)
}

func TestSystem_RaftConsistentGet(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	write := n.execute(t, leader, clusterstore.Command{Operation: clusterstore.Put, Key: "owner", Value: []byte("node-a")})
	require.NoError(t, write.Err)
	n.committed(t, write.Index, 1, 2, 3)
	before := n.status(t, leader)
	for id := uint64(1); id <= 3; id++ {
		record := n.consistentGet(t, id, "owner")
		if id == leader {
			require.NoError(t, record.Err)
			require.True(t, record.Exists)
			require.Equal(t, []byte("node-a"), record.Value)
			require.Equal(t, write.Index, record.Revision)
			require.GreaterOrEqual(t, record.Index, write.Index)
			require.Equal(t, before.Term, record.Term)
			record.Value[0] = 'x'
		} else {
			require.ErrorIs(t, record.Err, clusterstore.ErrRaftNotLeader)
			require.Zero(t, record.Term)
			require.Empty(t, record.Value)
		}
		local := n.get(t, id, "owner")
		require.Equal(t, []byte("node-a"), local.Value)
		require.Zero(t, local.Term)
	}
	missing := n.consistentGet(t, leader, "missing")
	require.NoError(t, missing.Err)
	require.False(t, missing.Exists)
	require.NotZero(t, missing.Term)
	result, err := n.systems[leader].Ask(n.refs[leader], clusterstore.RaftRuntimeCompact{}).Result()
	require.NoError(t, err)
	require.NoError(t, result.(clusterstore.RaftRuntimeResult).Err)
	record := n.consistentGet(t, leader, "owner")
	require.NoError(t, record.Err)
	require.Equal(t, []byte("node-a"), record.Value)
	after := n.status(t, leader)
	require.Equal(t, before.LastIndex, after.LastIndex, "读取不增加日志位置")
	require.Zero(t, after.PendingReads)
}

func TestSystem_RaftConsistentGetPartitionRecovery(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	old := n.leader(t, 1, 2, 3)
	first := n.execute(t, old, clusterstore.Command{Operation: clusterstore.Put, Key: "owner", Value: []byte("old")})
	require.NoError(t, first.Err)
	n.committed(t, first.Index, 1, 2, 3)
	n.partition(old)
	var majority []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != old {
			majority = append(majority, id)
		}
	}
	leader := n.leader(t, majority...)
	updated := n.execute(t, leader, clusterstore.Command{Operation: clusterstore.Put, Key: "owner", Value: []byte("new"), ExpectedRevision: first.Index})
	require.NoError(t, updated.Err)
	require.True(t, updated.Result.Applied)
	result, err := n.systems[old].Ask(n.refs[old], clusterstore.RaftRuntimeGet{Key: "owner", Timeout: 80 * time.Millisecond}, 2*time.Second).Result()
	require.NoError(t, err)
	expired := result.(clusterstore.RaftRuntimeRecord)
	require.ErrorIs(t, expired.Err, clusterstore.ErrRaftReadTimeout, "仍自认 Leader 的少数派不能用旧确认完成新读取")
	require.Empty(t, expired.Value)
	require.Equal(t, []byte("old"), n.get(t, old, "owner").Value)
	current := n.consistentGet(t, leader, "owner")
	require.NoError(t, current.Err)
	require.Equal(t, []byte("new"), current.Value)
	pending := n.systems[old].Ask(n.refs[old], clusterstore.RaftRuntimeGet{Key: "owner", Timeout: time.Minute}, 3*time.Second)
	require.Eventually(t, func() bool { return n.status(t, old).PendingReads == 1 }, time.Second, 5*time.Millisecond)
	n.partition()
	result, err = pending.Result()
	require.NoError(t, err)
	require.ErrorIs(t, result.(clusterstore.RaftRuntimeRecord).Err, clusterstore.ErrRaftNotLeader)
	n.committed(t, updated.Index, 1, 2, 3)
	for id := uint64(1); id <= 3; id++ {
		require.Zero(t, n.status(t, id).PendingReads)
		require.Equal(t, []byte("new"), n.get(t, id, "owner").Value)
	}
	again, err := pending.Result()
	require.NoError(t, err)
	require.ErrorIs(t, again.(clusterstore.RaftRuntimeRecord).Err, clusterstore.ErrRaftNotLeader)
}

func TestSystem_RaftConsistentGetActorStopped(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	n.partition(leader)
	pending := n.systems[leader].Ask(n.refs[leader], clusterstore.RaftRuntimeGet{Key: "owner", Timeout: time.Minute}, 2*time.Second)
	require.Eventually(t, func() bool { return n.status(t, leader).PendingReads == 1 }, time.Second, 5*time.Millisecond)
	n.systems[leader].Kill(n.refs[leader], false)
	result, err := pending.Result()
	require.NoError(t, err)
	require.ErrorIs(t, result.(clusterstore.RaftRuntimeRecord).Err, clusterstore.ErrRaftRuntimeClosed)
}

func TestSystem_RaftConsistentGetConcurrentWrites(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	var revision uint64
	for iteration := range 8 {
		value := []byte{byte(iteration)}
		write := n.execute(t, leader, clusterstore.Command{Operation: clusterstore.Put, Key: "owner", Value: value, ExpectedRevision: revision})
		require.NoError(t, write.Err)
		require.True(t, write.Result.Applied)
		revision = write.Result.Revision
		type answer struct {
			result any
			err    error
		}
		answers := make(chan answer, 12)
		for range 12 {
			go func() {
				result, err := n.systems[leader].Ask(n.refs[leader], clusterstore.RaftRuntimeGet{Key: "owner"}, 2*time.Second).Result()
				answers <- answer{result, err}
			}()
		}
		for range 12 {
			answer := receiveRetryResult(t, answers)
			require.NoError(t, answer.err)
			record := answer.result.(clusterstore.RaftRuntimeRecord)
			require.NoError(t, record.Err)
			require.Equal(t, value, record.Value, "已返回成功的写入必须被后开始的读取观察到")
			require.Equal(t, revision, record.Revision)
		}
	}
}
