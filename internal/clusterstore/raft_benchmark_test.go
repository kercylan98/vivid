package clusterstore

import (
	"fmt"
	"os"
	"testing"
)

func BenchmarkRaftStorage_Save(b *testing.B) {
	for _, batch := range []int{1, 32} {
		b.Run(fmt.Sprint(batch), func(b *testing.B) {
			storage, path := createRaftStorage(b)
			defer func() { storage.Close() }()
			update := RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1}, Entries: make([]RaftEntry, batch)}
			for i := range update.Entries {
				update.Entries[i].Term = 1
				update.Entries[i].Data = make([]byte, 64)
			}
			b.ReportAllocs()
			b.SetBytes(int64(batch * 64))
			for b.Loop() {
				last := len(storage.state.entries)
				if last+batch > maxRaftEntries {
					b.StopTimer()
					if err := storage.Close(); err != nil {
						b.Fatal(err)
					}
					if err := os.Remove(path); err != nil {
						b.Fatal(err)
					}
					next, err := CreateRaftStorage(path, raftTestConfig())
					if err != nil {
						b.Fatal(err)
					}
					storage, last = next, 0
					b.StartTimer()
				}
				for i := range update.Entries {
					update.Entries[i].Index = uint64(last + i + 1)
				}
				update.HardState.Commit = uint64(last + batch)
				if err := storage.Save(update); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRaftStorage_Recover(b *testing.B) {
	storage, path := createRaftStorage(b)
	update := RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1, Commit: 1000}, Entries: make([]RaftEntry, 1000)}
	for i := range update.Entries {
		update.Entries[i] = RaftEntry{Index: uint64(i + 1), Term: 1, Data: make([]byte, 64)}
	}
	if err := storage.Save(update); err != nil {
		b.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		recovered, err := OpenRaftStorage(path, raftTestConfig())
		if err != nil {
			b.Fatal(err)
		}
		if err := recovered.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
