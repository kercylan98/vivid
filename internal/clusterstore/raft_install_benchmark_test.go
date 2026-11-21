package clusterstore

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// BenchmarkRaftInstall 测量完整快照安装与刷盘，不包含生成快照或传输。
func BenchmarkRaftInstall(b *testing.B) {
	for _, count := range []int{1000, 10000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			storage, err := CreateRaftStorage(filepath.Join(b.TempDir(), "install.wal"), raftTestConfig())
			require.NoError(b, err)
			defer storage.Close()
			require.NoError(b, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 1}}))
			var store Store
			for i := range count {
				_, err := store.Apply(uint64(i+1), Command{Operation: Put, Key: "singleton/" + strconv.Itoa(i), Value: make([]byte, 64)})
				require.NoError(b, err)
			}
			data := store.Snapshot()
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				b.StopTimer()
				_, err := store.Apply(store.Index()+1, Command{})
				require.NoError(b, err)
				data = store.Snapshot()
				snapshot := RaftSnapshot{Index: store.Index(), Term: 1, Data: data}
				b.StartTimer()
				if err := storage.InstallSnapshot(snapshot); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
