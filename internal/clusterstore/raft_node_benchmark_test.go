package clusterstore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkRaftNode_Commit 逐条提案并送达全部确认和提交通知，不测流水线吞吐。
// Memory 使用相同持久状态约束但不执行 I/O；WAL 包含三个本地文件的同步刷盘，不含网络。
func BenchmarkRaftNode_Commit(b *testing.B) {
	for _, mode := range []string{"Memory", "WAL"} {
		b.Run(mode, func(b *testing.B) {
			directory := b.TempDir()
			var nodes []*RaftNode
			var disks []*RaftStorage
			closeDisks := func() {
				for _, disk := range disks {
					if err := disk.Close(); err != nil {
						b.Fatal(err)
					}
				}
				disks = nil
			}
			defer closeDisks()
			drain := func(queue []RaftMessage) {
				for len(queue) > 0 {
					m := queue[0]
					queue = queue[1:]
					out, err := nodes[m.To].Step(m)
					if err != nil {
						b.Fatal(err)
					}
					queue = append(queue, out...)
				}
			}
			reset := func() {
				closeDisks()
				nodes = make([]*RaftNode, 4)
				for id := uint64(1); id <= 3; id++ {
					config := RaftConfig{GroupID: "benchmark", NodeID: id, Voters: []uint64{1, 2, 3}}
					var storage raftNodeStorage = &memoryRaftStorage{config: config}
					if mode == "WAL" {
						path := filepath.Join(directory, fmt.Sprint(id)+".wal")
						if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
							b.Fatal(err)
						}
						disk, err := CreateRaftStorage(path, config)
						if err != nil {
							b.Fatal(err)
						}
						disks = append(disks, disk)
						storage = disk
					}
					var err error
					nodes[id], err = newRaftNode(storage)
					if err != nil {
						b.Fatal(err)
					}
				}
				out, err := nodes[1].Campaign()
				if err != nil {
					b.Fatal(err)
				}
				drain(out)
			}
			reset()
			data := make([]byte, 64)
			last := uint64(1)
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				if last >= maxRaftEntries-1 {
					b.StopTimer()
					reset()
					b.StartTimer()
				}
				index, out, err := nodes[1].Propose(data)
				if err != nil {
					b.Fatal(err)
				}
				drain(out)
				for id := 1; id <= 3; id++ {
					if nodes[id].hard.Commit != index {
						b.Fatal("未完成全部副本提交")
					}
				}
				last = index
			}
		})
	}
}
