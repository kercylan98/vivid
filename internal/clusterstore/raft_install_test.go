package clusterstore

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func remoteSnapshot(index, term uint64) RaftSnapshot {
	store := Store{index: index}
	return RaftSnapshot{Index: index, Term: term, Data: store.Snapshot()}
}

func installFixture(t testing.TB, path string) *RaftStorage {
	t.Helper()
	storage, err := CreateRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	require.NoError(t, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 4, Vote: 2, Commit: 1}, Entries: []RaftEntry{
		{Index: 1, Term: 1, Data: []byte("committed")}, {Index: 2, Term: 2},
		{Index: 3, Term: 2}, {Index: 4, Term: 3, Data: []byte("suffix")},
	}}))
	return storage
}

func TestRaftInstall_Recovery(t *testing.T) {
	for _, test := range []struct {
		name  string
		index uint64
		term  uint64
		last  uint64
	}{
		{"matching", 3, 2, 4}, {"conflict", 3, 3, 3}, {"beyond", 8, 3, 8},
		{"maximum", ^uint64(0), 3, ^uint64(0)},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "install.wal")
			storage := installFixture(t, path)
			defer storage.Close()
			snapshot := remoteSnapshot(test.index, test.term)
			original := bytes.Clone(snapshot.Data)
			require.NoError(t, storage.InstallSnapshot(snapshot))
			clear(snapshot.Data)
			require.Equal(t, RaftHardState{Term: 4, Vote: 2, Commit: test.index}, storage.state.hard)
			last, err := storage.LastIndex()
			require.NoError(t, err)
			require.Equal(t, test.last, last)
			require.NoError(t, storage.Close())
			restored, err := OpenRaftStorage(path, raftTestConfig())
			require.NoError(t, err)
			defer restored.Close()
			actual, err := restored.Snapshot()
			require.NoError(t, err)
			require.Equal(t, RaftSnapshot{Index: test.index, Term: test.term, Data: original}, actual)
			require.Equal(t, RaftHardState{Term: 4, Vote: 2, Commit: test.index}, restored.state.hard)
			if test.name == "matching" {
				entry, err := restored.Entry(4)
				require.NoError(t, err)
				require.Equal(t, []byte("suffix"), entry.Data)
			}
			if test.last != ^uint64(0) {
				hard := restored.state.hard
				hard.Commit = test.last + 1
				require.NoError(t, restored.Save(RaftUpdate{HardState: hard, Entries: []RaftEntry{{Index: hard.Commit, Term: 4}}}))
			}
		})
	}
}

func TestRaftInstall_Invalid(t *testing.T) {
	storage := installFixture(t, filepath.Join(t.TempDir(), "invalid.wal"))
	defer storage.Close()
	for _, snapshot := range []RaftSnapshot{
		{}, remoteSnapshot(1, 1), remoteSnapshot(3, 5),
		{Index: 3, Term: 2, Data: remoteSnapshot(2, 2).Data},
		{Index: 3, Term: 2, Data: []byte("bad")},
	} {
		err := storage.InstallSnapshot(snapshot)
		require.Error(t, err)
		require.EqualValues(t, 1, storage.state.hard.Commit)
		require.Zero(t, storage.state.baseIndex)
		require.Len(t, storage.state.entries, 4)
	}
	hard := storage.state.hard
	hard.Commit = 2
	require.NoError(t, storage.Save(RaftUpdate{HardState: hard}))
	require.ErrorIs(t, storage.InstallSnapshot(remoteSnapshot(3, 1)), ErrInvalidRaftUpdate, "快照末尾任期不能早于已有提交前缀")
	require.NoError(t, storage.InstallSnapshot(remoteSnapshot(3, 2)))
	require.ErrorIs(t, storage.InstallSnapshot(remoteSnapshot(3, 2)), ErrInvalidRaftUpdate)
	require.NoError(t, storage.Close())
	require.ErrorIs(t, storage.InstallSnapshot(remoteSnapshot(5, 3)), ErrJournalClosed)
}

func TestRaftInstall_Interrupted(t *testing.T) {
	for _, stage := range []string{"snapshot", "candidate", "closed", "published", "installed"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "interrupted.wal")
			storage := installFixture(t, path)
			failure := errors.New("interrupted remote snapshot")
			err := storage.installSnapshot(remoteSnapshot(3, 3), func(current string) error {
				other, err := OpenRaftStorage(path, raftTestConfig())
				require.ErrorIs(t, err, ErrJournalInUse)
				require.Nil(t, other)
				if current == stage {
					return failure
				}
				return nil
			})
			require.ErrorIs(t, err, failure)
			require.NoError(t, storage.Close())
			restored, err := OpenRaftStorage(path, raftTestConfig())
			require.NoError(t, err)
			defer restored.Close()
			if stage == "published" || stage == "installed" {
				require.EqualValues(t, 3, restored.state.baseIndex)
				require.EqualValues(t, 3, restored.state.hard.Commit)
				require.Empty(t, restored.state.entries)
			} else {
				require.Zero(t, restored.state.baseIndex)
				require.EqualValues(t, 1, restored.state.hard.Commit)
				require.Len(t, restored.state.entries, 4)
			}
			require.EqualValues(t, 4, restored.state.hard.Term)
			require.EqualValues(t, 2, restored.state.hard.Vote)
			require.NoError(t, restored.Reclaim())
		})
	}
}

func TestRaftInstall_ProcessHelper(t *testing.T) {
	path := os.Getenv("VIVID_INSTALL_CRASH_PATH")
	if path == "" {
		return
	}
	storage := installFixture(t, path)
	err := storage.installSnapshot(remoteSnapshot(8, 3), func(stage string) error {
		if stage == os.Getenv("VIVID_INSTALL_CRASH_STAGE") {
			fmt.Println("ready")
			<-time.After(time.Minute)
		}
		return nil
	})
	t.Fatal("helper was not terminated", err)
}

func TestRaftInstall_ProcessCrash(t *testing.T) {
	for _, stage := range []string{"snapshot", "candidate", "closed", "published", "installed"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "crash.wal")
			command := exec.Command(os.Args[0], "-test.run=^TestRaftInstall_ProcessHelper$", "-test.timeout=30s")
			command.Env = append(os.Environ(), "VIVID_INSTALL_CRASH_PATH="+path, "VIVID_INSTALL_CRASH_STAGE="+stage)
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
				t.Fatal("helper did not reach snapshot stage")
			}
			require.NoError(t, command.Process.Kill())
			require.Error(t, command.Wait())
			restored, err := OpenRaftStorage(path, raftTestConfig())
			require.NoError(t, err)
			defer restored.Close()
			if stage == "published" || stage == "installed" {
				require.EqualValues(t, 8, restored.state.baseIndex)
				require.EqualValues(t, 8, restored.state.hard.Commit)
				require.Empty(t, restored.state.entries)
			} else {
				require.Zero(t, restored.state.baseIndex)
				require.EqualValues(t, 1, restored.state.hard.Commit)
				require.Len(t, restored.state.entries, 4)
			}
			require.EqualValues(t, 4, restored.state.hard.Term)
			require.EqualValues(t, 2, restored.state.hard.Vote)
			require.NoError(t, restored.Reclaim())
		})
	}
}

func TestRaftInstall_LargePayloadAndSuffix(t *testing.T) {
	storage := installFixture(t, filepath.Join(t.TempDir(), "large.wal"))
	defer storage.Close()
	for index := uint64(5); index <= 8; index++ {
		require.NoError(t, storage.Save(RaftUpdate{HardState: storage.state.hard, Entries: []RaftEntry{{Index: index, Term: 3, Data: bytes.Repeat([]byte{'s'}, 1536<<10)}}}))
	}
	var store Store
	for i := range 5 {
		_, err := store.Apply(uint64(i+1), Command{Operation: Put, Key: fmt.Sprint(i), Value: make([]byte, maxValueBytes)})
		require.NoError(t, err)
	}
	snapshot := RaftSnapshot{Index: 5, Term: 3, Data: store.Snapshot()}
	require.Greater(t, len(snapshot.Data), maxJournalRecordBytes)
	require.NoError(t, storage.InstallSnapshot(snapshot))
	actual, err := storage.Snapshot()
	require.NoError(t, err)
	restore, err := Restore(actual.Data)
	require.NoError(t, err)
	record, ok := restore.Get("4")
	require.True(t, ok)
	require.Len(t, record.Value, maxValueBytes)
	require.Len(t, storage.state.entries, 3)
	// 一次安装保留多批次后缀时，不能将其未提交部分提前标成提交。
	require.EqualValues(t, 5, storage.state.hard.Commit)
	digest := sha256.Sum256(actual.Data)
	require.Equal(t, digest, storage.snapshot)
}
