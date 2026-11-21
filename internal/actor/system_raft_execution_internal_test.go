package actor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func (n *raftRuntimeNetwork) execute(t *testing.T, id uint64, command clusterstore.Command) clusterstore.RaftRuntimeExecution {
	t.Helper()
	result, err := n.systems[id].Ask(n.refs[id], clusterstore.RaftRuntimeExecute{Command: command}, 6*time.Second).Result()
	require.NoError(t, err)
	return result.(clusterstore.RaftRuntimeExecution)
}

func TestSystem_RaftExecutionResults(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	first := n.execute(t, leader, clusterstore.Command{Operation: clusterstore.Put, Key: "owner", Value: []byte("node-a")})
	require.NoError(t, first.Err)
	require.Equal(t, clusterstore.Result{Applied: true, Revision: first.Index}, first.Result)
	require.GreaterOrEqual(t, n.status(t, leader).Applied, first.Index)
	conflict := n.execute(t, leader, clusterstore.Command{Operation: clusterstore.Put, Key: "owner", Value: []byte("node-b")})
	require.NoError(t, conflict.Err)
	require.Equal(t, clusterstore.Result{Revision: first.Index}, conflict.Result)
	require.Greater(t, conflict.Index, first.Index)
	n.committed(t, conflict.Index, 1, 2, 3)
	for id := uint64(1); id <= 3; id++ {
		require.Zero(t, n.status(t, id).PendingExecutions)
		require.Equal(t, []byte("node-a"), n.get(t, id, "owner").Value)
		if id != leader {
			refused := n.execute(t, id, clusterstore.Command{Operation: clusterstore.Delete, Key: "owner", ExpectedRevision: first.Index})
			require.ErrorIs(t, refused.Err, clusterstore.ErrRaftNotLeader)
			require.NotErrorIs(t, refused.Err, clusterstore.ErrRaftOutcomeUnknown)
			require.Zero(t, refused.Index)
		}
	}
}

func TestSystem_RaftExecutionPartitionOutcomes(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	old := n.leader(t, 1, 2, 3)
	first := n.execute(t, old, clusterstore.Command{Operation: clusterstore.Put, Key: "base", Value: []byte("committed")})
	require.NoError(t, first.Err)
	n.committed(t, first.Index, 1, 2, 3)
	n.partition(old)
	timed := n.systems[old].Ask(n.refs[old], clusterstore.RaftRuntimeExecute{
		Command: clusterstore.Command{Operation: clusterstore.Put, Key: "timed", Value: []byte("minority")}, Timeout: 80 * time.Millisecond,
	}, 2*time.Second)
	pending := n.systems[old].Ask(n.refs[old], clusterstore.RaftRuntimeExecute{
		Command: clusterstore.Command{Operation: clusterstore.Put, Key: "pending", Value: []byte("minority")}, Timeout: time.Minute,
	}, 5*time.Second)
	require.Eventually(t, func() bool { return n.status(t, old).PendingExecutions != 0 }, time.Second, 5*time.Millisecond)
	require.Equal(t, []byte("committed"), n.get(t, old, "base").Value, "执行等待不能阻塞 Actor 邮箱")
	result, err := timed.Result()
	require.NoError(t, err)
	expired := result.(clusterstore.RaftRuntimeExecution)
	require.ErrorIs(t, expired.Err, clusterstore.ErrRaftOutcomeUnknown)
	require.ErrorIs(t, expired.Err, clusterstore.ErrRaftExecutionTimeout)
	require.Greater(t, expired.Index, n.status(t, old).Commit)
	require.False(t, n.get(t, old, "timed").Exists)
	var majority []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != old {
			majority = append(majority, id)
		}
	}
	leader := n.leader(t, majority...)
	accepted := n.execute(t, leader, clusterstore.Command{Operation: clusterstore.Put, Key: "new", Value: []byte("majority")})
	require.NoError(t, accepted.Err)
	n.partition()
	result, err = pending.Result()
	require.NoError(t, err)
	unknown := result.(clusterstore.RaftRuntimeExecution)
	require.ErrorIs(t, unknown.Err, clusterstore.ErrRaftOutcomeUnknown)
	require.NotErrorIs(t, unknown.Err, clusterstore.ErrRaftExecutionTimeout, "任期变更及时结束等待，不必等满一分钟")
	require.Equal(t, clusterstore.Result{}, unknown.Result)
	n.committed(t, accepted.Index, 1, 2, 3)
	for id := uint64(1); id <= 3; id++ {
		require.Zero(t, n.status(t, id).PendingExecutions)
		require.False(t, n.get(t, id, "pending").Exists)
		require.Equal(t, []byte("majority"), n.get(t, id, "new").Value)
	}
	again, err := timed.Result()
	require.NoError(t, err)
	require.Equal(t, expired, again.(clusterstore.RaftRuntimeExecution))
}

func TestSystem_RaftExecutionActorStopped(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	n.partition(leader)
	pending := n.systems[leader].Ask(n.refs[leader], clusterstore.RaftRuntimeExecute{
		Command: clusterstore.Command{Operation: clusterstore.Put, Key: "owner"}, Timeout: time.Minute,
	}, 2*time.Second)
	require.Eventually(t, func() bool { return n.status(t, leader).PendingExecutions == 1 }, time.Second, 5*time.Millisecond)
	n.systems[leader].Kill(n.refs[leader], false, "execution stop test")
	result, err := pending.Result()
	require.NoError(t, err)
	require.ErrorIs(t, result.(clusterstore.RaftRuntimeExecution).Err, clusterstore.ErrRaftOutcomeUnknown)
	require.ErrorIs(t, result.(clusterstore.RaftRuntimeExecution).Err, clusterstore.ErrRaftRuntimeClosed)
}

func BenchmarkSystem_RaftExecute(b *testing.B) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	require.NoError(b, system.Start())
	defer func() { require.NoError(b, system.Stop(3*time.Second)) }()
	ref, err := NewRef(system.Ref().GetAddress(), "/raft")
	require.NoError(b, err)
	driver, err := clusterstore.NewRaftActor(clusterstore.RaftActorOptions{
		Path: filepath.Join(b.TempDir(), "node.wal"), Config: clusterstore.RaftConfig{GroupID: "bench", NodeID: 1, Voters: []uint64{1}},
		Create: true, Routes: map[uint64]vivid.ActorRef{1: ref}, HeartbeatInterval: time.Millisecond, ElectionMin: 10 * time.Millisecond, ElectionMax: 20 * time.Millisecond,
	})
	require.NoError(b, err)
	refValue, err := system.ActorOf(driver, vivid.WithActorName("raft"))
	require.NoError(b, err)
	require.Eventually(b, func() bool {
		result, err := system.Ask(refValue, clusterstore.RaftRuntimeStatusRequest{}, time.Second).Result()
		require.NoError(b, err)
		return result.(clusterstore.RaftRuntimeStatus).Role == clusterstore.RaftLeader
	}, time.Second, time.Millisecond)
	var revision uint64
	command := clusterstore.Command{Operation: clusterstore.Put, Key: "owner", Value: make([]byte, 64)}
	b.ReportAllocs()
	for b.Loop() {
		command.ExpectedRevision = revision
		result, err := system.Ask(refValue, clusterstore.RaftRuntimeExecute{Command: command}, time.Second).Result()
		if err != nil {
			b.Fatal(err)
		}
		executed := result.(clusterstore.RaftRuntimeExecution)
		if executed.Err != nil || executed.Result.Err != nil || !executed.Result.Applied {
			b.Fatalf("执行失败: %+v", executed)
		}
		revision = executed.Result.Revision
		if revision%1024 == 0 {
			b.StopTimer()
			result, err := system.Ask(refValue, clusterstore.RaftRuntimeCompact{}, time.Second).Result()
			require.NoError(b, err)
			require.NoError(b, result.(clusterstore.RaftRuntimeResult).Err)
			b.StartTimer()
		}
	}
}
