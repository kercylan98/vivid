package clusterstore

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func checkpointFixture(t testing.TB, path string) (*RaftStorage, [][]byte) {
	t.Helper()
	storage, err := CreateRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	update := RaftUpdate{HardState: RaftHardState{Term: 2, Vote: 1, Commit: 3}, Entries: []RaftEntry{
		{Index: 1, Term: 1, Data: []byte("first")}, {Index: 2, Term: 1},
		{Index: 3, Term: 2, Data: []byte("committed suffix")},
		{Index: 4, Term: 2, Data: []byte("uncommitted suffix")},
	}}
	require.NoError(t, storage.Save(update))
	var store Store
	snapshots := [][]byte{store.Snapshot()}
	for i := range 4 {
		result, err := store.Apply(uint64(i+1), Command{})
		require.NoError(t, err)
		require.True(t, result.Applied)
		snapshots = append(snapshots, store.Snapshot())
	}
	return storage, snapshots
}

func TestRaftCheckpoint_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raft.wal")
	storage, snapshots := checkpointFixture(t, path)
	t.Cleanup(func() { storage.Close() })
	originalHard, before := storage.state.hard, raftEntries(storage)
	snapshot, err := storage.Snapshot()
	require.NoError(t, err)
	require.Zero(t, snapshot)
	require.NoError(t, storage.Compact(snapshots[2]))
	require.Equal(t, originalHard, storage.state.hard)
	require.Equal(t, before[2:], raftEntries(storage))
	require.EqualValues(t, 2, storage.state.baseIndex)
	require.EqualValues(t, 1, storage.state.baseTerm)
	require.Equal(t, len(before[2].Data)+len(before[3].Data), storage.state.bytes)
	for _, index := range []uint64{0, 1, 2} {
		_, err := storage.Entry(index)
		require.ErrorIs(t, err, ErrRaftCompacted)
	}
	_, err = storage.Term(1)
	require.ErrorIs(t, err, ErrRaftCompacted)
	term, err := storage.Term(2)
	require.NoError(t, err)
	require.EqualValues(t, 1, term)
	term, err = storage.Term(3)
	require.NoError(t, err)
	require.EqualValues(t, 2, term)
	_, err = storage.Term(5)
	require.ErrorIs(t, err, ErrRaftEntryMissing)
	last, err := storage.LastIndex()
	require.NoError(t, err)
	require.EqualValues(t, 4, last)
	snapshot, err = storage.Snapshot()
	require.NoError(t, err)
	require.Equal(t, RaftSnapshot{Index: 2, Term: 1, Data: snapshots[2]}, snapshot)
	snapshot.Data[0] ^= 1
	require.NoError(t, storage.Close())
	storage, err = OpenRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	require.Equal(t, before[2:], raftEntries(storage))
	require.Equal(t, originalHard, storage.state.hard)
	require.NoError(t, storage.Save(RaftUpdate{HardState: originalHard, Entries: []RaftEntry{{Index: 5, Term: 2, Data: []byte("next")}}}))
	require.ErrorIs(t, storage.Save(RaftUpdate{HardState: originalHard, Entries: []RaftEntry{{Index: 2, Term: 2}}}), ErrInvalidRaftUpdate)
	require.ErrorIs(t, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 3, Commit: 3}, Entries: []RaftEntry{{Index: 3, Term: 3}}}), ErrInvalidRaftUpdate)
	oldSnapshot := storage.snapshotPath(storage.snapshot)
	require.NoError(t, storage.Compact(snapshots[3]))
	_, err = os.Stat(oldSnapshot)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Len(t, storage.state.entries, 2)
	require.EqualValues(t, 3, storage.state.baseIndex)
	require.NoError(t, storage.Close())
	storage, err = OpenRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	snapshot, err = storage.Snapshot()
	require.NoError(t, err)
	require.Equal(t, RaftSnapshot{Index: 3, Term: 2, Data: snapshots[3]}, snapshot)
}

func TestRaftCheckpoint_Invalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raft.wal")
	storage, snapshots := checkpointFixture(t, path)
	defer storage.Close()
	term, err := storage.Term(0)
	require.NoError(t, err)
	require.Zero(t, term)
	for _, index := range []int{0, 4} {
		require.ErrorIs(t, storage.Compact(snapshots[index]), ErrInvalidRaftUpdate)
	}
	invalid := bytes.Clone(snapshots[2])
	invalid[0] ^= 1
	require.ErrorIs(t, storage.Compact(invalid), ErrInvalidSnapshot)
	require.NoError(t, storage.Compact(snapshots[2]))
	require.ErrorIs(t, storage.Compact(snapshots[1]), ErrInvalidRaftUpdate)
	require.ErrorIs(t, storage.Compact(snapshots[2]), ErrInvalidRaftUpdate)
	require.NoError(t, storage.Close())
	_, err = storage.Snapshot()
	require.ErrorIs(t, err, ErrJournalClosed)
	_, err = storage.Term(2)
	require.ErrorIs(t, err, ErrJournalClosed)
	require.ErrorIs(t, storage.Compact(snapshots[3]), ErrJournalClosed)
	require.ErrorIs(t, storage.Reclaim(), ErrJournalClosed)
}

func TestRaftCheckpoint_Interrupted(t *testing.T) {
	for _, stage := range []string{"snapshot", "candidate", "closed", "published", "installed"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "raft.wal")
			storage, snapshots := checkpointFixture(t, path)
			failure := errors.New("interrupted installation")
			err := storage.compact(snapshots[2], func(current string) error {
				other, err := OpenRaftStorage(path, raftTestConfig())
				require.ErrorIs(t, err, ErrJournalInUse, "切换所有阶段仍持有固定锁")
				require.Nil(t, other)
				if current == stage {
					return failure
				}
				return nil
			})
			require.ErrorIs(t, err, failure)
			_, stateErr := storage.State()
			if stage == "closed" || stage == "published" {
				require.ErrorIs(t, stateErr, ErrJournalFailed)
			} else {
				require.NoError(t, stateErr)
			}
			require.NoError(t, storage.Close())
			recovered, err := OpenRaftStorage(path, raftTestConfig())
			require.NoError(t, err)
			defer recovered.Close()
			require.Equal(t, RaftHardState{Term: 2, Vote: 1, Commit: 3}, recovered.state.hard)
			last, err := recovered.LastIndex()
			require.NoError(t, err)
			require.EqualValues(t, 4, last)
			if stage == "published" || stage == "installed" {
				require.EqualValues(t, 2, recovered.state.baseIndex)
			} else {
				require.Zero(t, recovered.state.baseIndex)
			}
			require.NoError(t, recovered.Reclaim())
			_, err = recovered.Entry(4)
			require.NoError(t, err)
		})
	}
}

func TestRaftCheckpoint_CorruptSnapshot(t *testing.T) {
	for _, kind := range []string{"missing", "checksum", "index", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "raft.wal")
			storage, snapshots := checkpointFixture(t, path)
			require.NoError(t, storage.Compact(snapshots[2]))
			snapshotPath := storage.snapshotPath(storage.snapshot)
			require.NoError(t, storage.Close())
			switch kind {
			case "missing":
				require.NoError(t, os.Remove(snapshotPath))
			case "checksum":
				data := bytes.Clone(snapshots[2])
				data[0] ^= 1
				require.NoError(t, os.WriteFile(snapshotPath, data, 0600))
			case "index":
				require.NoError(t, os.WriteFile(snapshotPath, snapshots[1], 0600))
			case "oversized":
				file, err := os.OpenFile(snapshotPath, os.O_RDWR, 0)
				require.NoError(t, err)
				require.NoError(t, file.Truncate(maxSnapshotBytes+1))
				require.NoError(t, file.Close())
			}
			recovered, err := OpenRaftStorage(path, raftTestConfig())
			require.Error(t, err, "不能跳过已选快照回退到旧日志")
			require.Nil(t, recovered)
		})
	}
}

func TestRaftCheckpoint_Reclaim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raft.wal")
	storage, snapshots := checkpointFixture(t, path)
	defer storage.Close()
	preserved := []string{path + ".snapshot-note", path + ".rewrite-" + strings.Repeat("z", 32), path + ".staging-12", path + ".notes", path + ".lock"}
	for _, name := range preserved[:len(preserved)-1] {
		require.NoError(t, os.WriteFile(name, []byte("unrelated"), 0600))
	}
	garbage := []string{path + ".snapshot-" + strings.Repeat("a", 64), path + ".rewrite-" + strings.Repeat("b", 32), path + ".staging-" + strings.Repeat("c", 32)}
	for _, name := range garbage {
		require.NoError(t, os.WriteFile(name, []byte("orphan"), 0600))
	}
	require.NoError(t, storage.Compact(snapshots[2]))
	for _, name := range garbage {
		_, err := os.Stat(name)
		require.ErrorIs(t, err, os.ErrNotExist)
	}
	for _, name := range preserved {
		_, err := os.Stat(name)
		require.NoError(t, err)
	}
	require.NoError(t, storage.Reclaim())
	require.NoError(t, storage.Reclaim())
	_, err := storage.Snapshot()
	require.NoError(t, err)
}

func TestRaftCheckpoint_Encoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raft.wal")
	storage, snapshots := checkpointFixture(t, path)
	defer storage.Close()
	digest := sha256.Sum256(snapshots[2])
	require.NoError(t, storage.writeSnapshot(snapshots[2], digest))
	valid := encodeRaftCheckpoint(2, 1, RaftHardState{Term: 2, Vote: 1, Commit: 2}, digest)
	for size := range len(valid) {
		target := &RaftStorage{path: path, config: storage.config}
		require.Error(t, target.restoreCheckpoint(valid[:size]))
	}
	for _, offset := range []int{9, 17, 25, 41} {
		data := bytes.Clone(valid)
		binary.LittleEndian.PutUint64(data[offset:], 0)
		target := &RaftStorage{path: path, config: storage.config}
		require.ErrorIs(t, target.restoreCheckpoint(data), ErrInvalidRaftState)
	}
	target := &RaftStorage{path: path, config: storage.config}
	wrongVote := bytes.Clone(valid)
	binary.LittleEndian.PutUint64(wrongVote[33:], 99)
	require.ErrorIs(t, target.restoreCheckpoint(wrongVote), ErrInvalidRaftState)
	wrongIndex := encodeRaftCheckpoint(3, 1, RaftHardState{Term: 2, Vote: 1, Commit: 3}, digest)
	require.ErrorIs(t, target.restoreCheckpoint(wrongIndex), ErrInvalidSnapshot)
	require.NoError(t, target.restoreCheckpoint(valid))
	journal, otherPath := createJournal(t)
	_, err := journal.Append(encodeRaftConfig(storage.config))
	require.NoError(t, err)
	_, err = journal.Append(encodeRaftUpdate(RaftUpdate{HardState: RaftHardState{Term: 2}}))
	require.NoError(t, err)
	_, err = journal.Append(valid)
	require.NoError(t, err)
	require.NoError(t, journal.Close())
	recovered, err := OpenRaftStorage(otherPath, raftTestConfig())
	require.ErrorIs(t, err, ErrInvalidRaftState)
	require.Nil(t, recovered)
}

func TestRaftCheckpoint_LargeAndHighIndex(t *testing.T) {
	storage, path := createRaftStorage(t)
	var store Store
	for i := range 5 {
		apply(t, &store, Command{Operation: Put, Key: fmt.Sprint(i), Value: make([]byte, maxValueBytes)})
	}
	require.NoError(t, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 1, Commit: 5}, Entries: []RaftEntry{{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1}, {Index: 4, Term: 1}, {Index: 5, Term: 1}}}))
	snapshot := store.Snapshot()
	require.Greater(t, len(snapshot), maxJournalRecordBytes, "快照不受单条 WAL 批次的大小限制")
	require.NoError(t, storage.Compact(snapshot))
	require.NoError(t, storage.Close())
	recovered, err := OpenRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	actual, err := recovered.Snapshot()
	require.NoError(t, err)
	require.Equal(t, snapshot, actual.Data)
	require.NoError(t, recovered.Close())
	// 基准超过旧位置上限，容量按条目数量计算；同时验证 uint64 回绕。
	highPath := filepath.Join(t.TempDir(), "high.wal")
	high, err := CreateRaftStorage(highPath, raftTestConfig())
	require.NoError(t, err)
	defer high.Close()
	base := ^uint64(0) - 1
	highStore := Store{index: base}
	highSnapshot := highStore.Snapshot()
	digest := sha256.Sum256(highSnapshot)
	require.NoError(t, high.writeSnapshot(highSnapshot, digest))
	_, err = high.journal.Append(encodeRaftCheckpoint(base, 1, RaftHardState{Term: 1, Commit: base}, digest))
	require.NoError(t, err)
	require.NoError(t, high.Close())
	high, err = OpenRaftStorage(highPath, raftTestConfig())
	require.NoError(t, err)
	defer high.Close()
	require.NoError(t, high.Save(RaftUpdate{HardState: RaftHardState{Term: 1, Commit: base + 1}, Entries: []RaftEntry{{Index: base + 1, Term: 1}}}))
	last, err := high.LastIndex()
	require.NoError(t, err)
	require.Equal(t, ^uint64(0), last)
	require.ErrorIs(t, high.Save(RaftUpdate{HardState: high.state.hard, Entries: []RaftEntry{{Index: 0, Term: 1}}}), ErrInvalidRaftUpdate)
	_, err = highStore.Apply(base+1, Command{})
	require.NoError(t, err)
	require.NoError(t, high.Compact(highStore.Snapshot()))
	require.NoError(t, high.Close())
	final, err := OpenRaftStorage(highPath, raftTestConfig())
	require.NoError(t, err)
	last, err = final.LastIndex()
	require.NoError(t, err)
	require.Equal(t, ^uint64(0), last)
	require.NoError(t, final.Close())
}

func TestRaftCheckpoint_ProcessHelper(t *testing.T) {
	path := os.Getenv("VIVID_CHECKPOINT_CRASH_PATH")
	if path == "" {
		return
	}
	storage, snapshots := checkpointFixture(t, path)
	err := storage.compact(snapshots[2], func(stage string) error {
		if stage == os.Getenv("VIVID_CHECKPOINT_CRASH_STAGE") {
			fmt.Println("ready")
			<-time.After(time.Minute)
		}
		return nil
	})
	t.Fatal("helper was not terminated", err)
}

func TestRaftCheckpoint_ProcessCrash(t *testing.T) {
	for _, stage := range []string{"snapshot", "candidate", "closed", "published", "installed"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "crash.wal")
			command := exec.Command(os.Args[0], "-test.run=^TestRaftCheckpoint_ProcessHelper$", "-test.timeout=30s")
			command.Env = append(os.Environ(), "VIVID_CHECKPOINT_CRASH_PATH="+path, "VIVID_CHECKPOINT_CRASH_STAGE="+stage)
			stdout, err := command.StdoutPipe()
			require.NoError(t, err)
			require.NoError(t, command.Start())
			t.Cleanup(func() {
				if command.ProcessState == nil {
					command.Process.Kill()
					command.Wait()
				}
			})
			ready := make(chan string, 1)
			go func() { line, _ := bufio.NewReader(stdout).ReadString('\n'); ready <- line }()
			select {
			case line := <-ready:
				require.Equal(t, "ready\n", line)
			case <-time.After(10 * time.Second):
				t.Fatal("helper did not reach checkpoint stage")
			}
			other, err := OpenRaftStorage(path, raftTestConfig())
			require.ErrorIs(t, err, ErrJournalInUse)
			require.Nil(t, other)
			require.NoError(t, command.Process.Kill())
			require.Error(t, command.Wait())
			recovered, err := OpenRaftStorage(path, raftTestConfig())
			require.NoError(t, err)
			defer recovered.Close()
			require.Equal(t, RaftHardState{Term: 2, Vote: 1, Commit: 3}, recovered.state.hard)
			last, err := recovered.LastIndex()
			require.NoError(t, err)
			require.EqualValues(t, 4, last)
			if stage == "published" || stage == "installed" {
				require.EqualValues(t, 2, recovered.state.baseIndex)
			} else {
				require.Zero(t, recovered.state.baseIndex)
			}
			require.NoError(t, recovered.Reclaim())
			require.NoError(t, recovered.Save(RaftUpdate{HardState: recovered.state.hard, Entries: []RaftEntry{{Index: 5, Term: 2}}}))
		})
	}
}
