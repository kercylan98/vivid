package clusterstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRaftCheckpoint_ReleasesCapacity(t *testing.T) {
	storage, path := createRaftStorage(t)
	entries := make([]RaftEntry, maxRaftEntries)
	for i := range entries {
		entries[i] = RaftEntry{Index: uint64(i + 1), Term: 1}
	}
	hard := RaftHardState{Term: 1, Vote: 1, Commit: maxRaftEntries}
	require.NoError(t, storage.Save(RaftUpdate{HardState: hard, Entries: entries}))
	before, err := os.Stat(path)
	require.NoError(t, err)
	var store Store
	for i := range maxRaftEntries {
		_, err := store.Apply(uint64(i+1), Command{})
		require.NoError(t, err)
	}
	require.NoError(t, storage.Compact(store.Snapshot()))
	after, err := os.Stat(path)
	require.NoError(t, err)
	require.Less(t, after.Size(), before.Size()/100, "旧批次已从恢复入口回收")
	require.Empty(t, storage.state.entries)
	require.Zero(t, cap(storage.state.entries), "压缩不保留旧条目数组")
	for range 3 {
		next := store.Index() + 1
		hard.Commit = next
		require.NoError(t, storage.Save(RaftUpdate{HardState: hard, Entries: []RaftEntry{{Index: next, Term: 1, Data: []byte("next")}}}))
		_, err := store.Apply(next, Command{})
		require.NoError(t, err)
		require.NoError(t, storage.Compact(store.Snapshot()))
	}
	require.NoError(t, storage.Close())
	recovered, err := OpenRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	defer recovered.Close()
	last, err := recovered.LastIndex()
	require.NoError(t, err)
	require.EqualValues(t, maxRaftEntries+3, last)
	require.Equal(t, hard, recovered.state.hard)
}

func TestRaftCheckpoint_MultipleSuffixBatches(t *testing.T) {
	storage, path := createRaftStorage(t)
	for i := range 4 {
		index := uint64(i + 1)
		hard := RaftHardState{Term: 2, Vote: 2, Commit: min(index, 3)}
		require.NoError(t, storage.Save(RaftUpdate{HardState: hard, Entries: []RaftEntry{{Index: index, Term: 2, Data: bytes.Repeat([]byte{byte(i)}, 1536<<10)}}}))
	}
	before := raftEntries(storage)
	var store Store
	apply(t, &store, Command{})
	require.NoError(t, storage.Compact(store.Snapshot()))
	require.Equal(t, before[1:], raftEntries(storage))
	require.Equal(t, RaftHardState{Term: 2, Vote: 2, Commit: 3}, storage.state.hard)
	require.NoError(t, storage.Close())
	recovered, err := OpenRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	defer recovered.Close()
	require.Equal(t, before[1:], raftEntries(recovered))
	require.Equal(t, RaftHardState{Term: 2, Vote: 2, Commit: 3}, recovered.state.hard)
}

func TestRaftCheckpoint_ReplaceFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raft.wal")
	storage, snapshots := checkpointFixture(t, path)
	err := storage.compact(snapshots[2], func(stage string) error {
		if stage == "closed" {
			files, err := filepath.Glob(path + ".rewrite-*")
			require.NoError(t, err)
			require.Len(t, files, 1)
			require.NoError(t, os.Remove(files[0]))
		}
		return nil
	})
	require.Error(t, err)
	_, err = storage.State()
	require.ErrorIs(t, err, ErrJournalFailed)
	require.ErrorIs(t, storage.Reclaim(), ErrJournalFailed)
	require.NoError(t, storage.Close())
	recovered, err := OpenRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	defer recovered.Close()
	require.Zero(t, recovered.state.baseIndex, "切换失败仍从原 WAL 恢复")
	require.Len(t, recovered.state.entries, 4)
	require.NoError(t, recovered.Reclaim())
}

func TestRaftCheckpoint_ReusesOrphanSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raft.wal")
	storage, snapshots := checkpointFixture(t, path)
	defer storage.Close()
	interrupted := errors.New("stop after snapshot publication")
	require.ErrorIs(t, storage.compact(snapshots[2], func(stage string) error {
		if stage == "snapshot" {
			return interrupted
		}
		return nil
	}), interrupted)
	require.NoError(t, storage.Compact(snapshots[2]), "重试能够验证并复用上次发布的独立快照")
	actual, err := storage.Snapshot()
	require.NoError(t, err)
	require.Equal(t, snapshots[2], actual.Data)
}

func TestRaftCheckpoint_RestoreBusinessState(t *testing.T) {
	storage, path := createRaftStorage(t)
	commands := []Command{
		{Operation: Put, Key: "orders", Value: []byte("node-a")},
		{Operation: Put, Key: "payments", Value: []byte("node-b")},
		{Operation: Put, Key: "orders", ExpectedRevision: 1, Value: []byte("node-c")},
		{Operation: Delete, Key: "orders", ExpectedRevision: 1}, // 旧版本比较失败仍推进位置
		{},
	}
	var state Store
	var snapshot []byte
	entries := make([]RaftEntry, len(commands))
	for i, command := range commands {
		apply(t, &state, command)
		if i == 1 {
			snapshot = state.Snapshot()
		}
		data, err := json.Marshal(command)
		require.NoError(t, err)
		entries[i] = RaftEntry{Index: uint64(i + 1), Term: 1, Data: data}
	}
	require.NoError(t, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 1, Commit: 5}, Entries: entries}))
	require.NoError(t, storage.Compact(snapshot))
	require.NoError(t, storage.Close())
	recovered, err := OpenRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	defer recovered.Close()
	checkpoint, err := recovered.Snapshot()
	require.NoError(t, err)
	restored, err := Restore(checkpoint.Data)
	require.NoError(t, err)
	hard, err := recovered.State()
	require.NoError(t, err)
	for index := checkpoint.Index + 1; index <= hard.Commit; index++ {
		entry, err := recovered.Entry(index)
		require.NoError(t, err)
		var command Command
		require.NoError(t, json.Unmarshal(entry.Data, &command))
		_, err = restored.Apply(index, command)
		require.NoError(t, err)
	}
	require.Equal(t, state.Snapshot(), restored.Snapshot(), "快照与已提交后缀重建完整业务状态")
}
