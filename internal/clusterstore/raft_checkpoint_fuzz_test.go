package clusterstore

import (
	"crypto/sha256"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzRaftCheckpoint_Restore(f *testing.F) {
	path := filepath.Join(f.TempDir(), "raft.wal")
	storage, snapshots := checkpointFixture(f, path)
	digest := sha256.Sum256(snapshots[2])
	require.NoError(f, storage.writeSnapshot(snapshots[2], digest))
	require.NoError(f, storage.Close())
	config, err := normalizeRaftConfig(raftTestConfig())
	require.NoError(f, err)
	valid := encodeRaftCheckpoint(2, 1, RaftHardState{Term: 2, Vote: 1, Commit: 2}, digest)
	f.Add(valid)
	f.Add([]byte{})
	f.Add(valid[:len(valid)-1])
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1024 {
			return
		}
		target := &RaftStorage{path: path, config: config}
		if err := target.restoreCheckpoint(data); err != nil {
			return
		}
		require.EqualValues(t, 2, target.state.baseIndex)
		require.Equal(t, target.state.baseIndex, target.state.hard.Commit)
		require.Greater(t, target.state.baseTerm, uint64(0))
		require.GreaterOrEqual(t, target.state.hard.Term, target.state.baseTerm)
		require.Equal(t, digest, target.snapshot)
		require.Empty(t, target.state.entries)
		change, err := target.prepare(RaftUpdate{HardState: target.state.hard})
		require.NoError(t, err)
		target.state.install(change)
		require.EqualValues(t, 2, target.state.lastIndex())
	})
}
