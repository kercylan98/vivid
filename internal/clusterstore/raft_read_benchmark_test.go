package clusterstore

import "testing"

// BenchmarkRaftRead_Quorum 测量三个内存节点的完整确认往返，不含 Actor、网络及磁盘。
func BenchmarkRaftRead_Quorum(b *testing.B) {
	network := newRaftNetwork(b, 3)
	network.campaign(b, 1)
	network.drain(b)
	leader := network.nodes[1]
	b.ReportAllocs()
	for b.Loop() {
		out, err := leader.beginRead()
		if err != nil {
			b.Fatal(err)
		}
		for _, probe := range out {
			response, err := network.nodes[probe.To].Step(probe)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := leader.Step(response[0]); err != nil {
				b.Fatal(err)
			}
		}
		if !leader.read.ready {
			b.Fatal("未完成多数派确认")
		}
		leader.cancelRead(leader.read.request)
	}
}
