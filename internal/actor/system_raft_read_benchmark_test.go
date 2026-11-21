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

func benchmarkRaftReadSystem(b *testing.B) (*System, vivid.ActorRef) {
	b.Helper()
	directory := b.TempDir()
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	require.NoError(b, system.Start())
	b.Cleanup(func() { require.NoError(b, system.Stop(3*time.Second)) })
	ref, err := NewRef(system.Ref().GetAddress(), "/raft")
	require.NoError(b, err)
	driver, err := clusterstore.NewRaftActor(clusterstore.RaftActorOptions{
		Path: filepath.Join(directory, "node.wal"), Config: clusterstore.RaftConfig{GroupID: "bench-read", NodeID: 1, Voters: []uint64{1}},
		Create: true, Routes: map[uint64]vivid.ActorRef{1: ref}, HeartbeatInterval: time.Millisecond, ElectionMin: 10 * time.Millisecond, ElectionMax: 20 * time.Millisecond,
	})
	require.NoError(b, err)
	spawned, err := system.ActorOf(driver, vivid.WithActorName("raft"))
	require.NoError(b, err)
	require.Eventually(b, func() bool {
		result, err := system.Ask(spawned, clusterstore.RaftRuntimeStatusRequest{}, time.Second).Result()
		require.NoError(b, err)
		return result.(clusterstore.RaftRuntimeStatus).Role == clusterstore.RaftLeader
	}, time.Second, time.Millisecond)
	result, err := system.Ask(spawned, clusterstore.RaftRuntimeExecute{Command: clusterstore.Command{Operation: clusterstore.Put, Key: "owner", Value: make([]byte, 64)}}).Result()
	require.NoError(b, err)
	require.NoError(b, result.(clusterstore.RaftRuntimeExecution).Err)
	return system, spawned
}

func BenchmarkSystem_RaftLocalGet(b *testing.B) {
	system, ref := benchmarkRaftReadSystem(b)
	b.ReportAllocs()
	for b.Loop() {
		result, err := system.Ask(ref, clusterstore.RaftRuntimeLocalGet{Key: "owner"}).Result()
		if err != nil {
			b.Fatal(err)
		}
		record := result.(clusterstore.RaftRuntimeRecord)
		if record.Err != nil || !record.Exists || len(record.Value) != 64 {
			b.Fatalf("读取失败: %+v", record)
		}
	}
}

// BenchmarkSystem_RaftConsistentGet 包含 Actor Ask 与返回副本，单节点仍检查本任期提交边界。
func BenchmarkSystem_RaftConsistentGet(b *testing.B) {
	system, ref := benchmarkRaftReadSystem(b)
	b.ReportAllocs()
	for b.Loop() {
		result, err := system.Ask(ref, clusterstore.RaftRuntimeGet{Key: "owner"}).Result()
		if err != nil {
			b.Fatal(err)
		}
		record := result.(clusterstore.RaftRuntimeRecord)
		if record.Err != nil || !record.Exists || len(record.Value) != 64 || record.Term == 0 {
			b.Fatalf("读取失败: %+v", record)
		}
	}
}
