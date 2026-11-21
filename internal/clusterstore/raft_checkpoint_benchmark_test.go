package clusterstore

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkRaftCheckpoint(b *testing.B) {
	for _, count := range []int{1000, 10000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			storage, err := CreateRaftStorage(filepath.Join(b.TempDir(), "checkpoint.wal"), raftTestConfig())
			require.NoError(b, err)
			defer storage.Close()
			var store Store
			value := make([]byte, 64)
			entries := make([]RaftEntry, count)
			for i := range count {
				_, err := store.Apply(uint64(i+1), Command{Operation: Put, Key: "singleton/" + strconv.Itoa(i), Value: value})
				require.NoError(b, err)
			}
			for i := range entries {
				entries[i] = RaftEntry{Index: uint64(i + 1), Term: 1, Data: value}
			}
			require.NoError(b, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1, Commit: uint64(count)}, Entries: entries}))
			data := store.Snapshot()
			require.NoError(b, storage.Compact(data))
			b.Run("compact", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					b.StopTimer()
					next := store.Index() + 1
					_, err := store.Apply(next, Command{})
					require.NoError(b, err)
					require.NoError(b, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1, Commit: next}, Entries: []RaftEntry{{Index: next, Term: 1}}}))
					snapshot := store.Snapshot()
					b.StartTimer()
					if err := storage.Compact(snapshot); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("recover", func(b *testing.B) {
				b.ReportAllocs()
				path := storage.path
				require.NoError(b, storage.Close())
				for b.Loop() {
					recovered, err := OpenRaftStorage(path, raftTestConfig())
					if err != nil {
						b.Fatal(err)
					}
					if err := recovered.Close(); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
