package clusterstore

import "testing"

// BenchmarkVirtualOwnership_Claim 包含归属准备、命令编解码、内存 CAS 与实际回执解析，不含 Raft、网络或磁盘。
func BenchmarkVirtualOwnership_Claim(b *testing.B) {
	owner := VirtualOwner{InstanceID: [16]byte{1}, Address: "node.example:9000"}
	var store Store
	var generation uint64
	b.ReportAllocs()
	for b.Loop() {
		claim, err := ClaimVirtualOwnership("counter", "one", owner, generation)
		if err != nil {
			b.Fatal(err)
		}
		data, err := EncodeCommand(claim.Request(0).Command)
		if err != nil {
			b.Fatal(err)
		}
		command, err := DecodeCommand(data)
		if err != nil {
			b.Fatal(err)
		}
		index := store.Index() + 1
		result, err := store.Apply(index, command)
		if err != nil {
			b.Fatal(err)
		}
		ownership, err := claim.Result(RaftRuntimeExecution{Index: index, Term: 1, Result: result})
		if err != nil {
			b.Fatal(err)
		}
		generation = ownership.Generation
	}
}

// BenchmarkVirtualOwnership_Read 包含本地记录副本与一致性回执解析，不包含多数派确认。
func BenchmarkVirtualOwnership_Read(b *testing.B) {
	owner := VirtualOwner{InstanceID: [16]byte{1}, Address: "node.example:9000"}
	claim, err := ClaimVirtualOwnership("counter", "one", owner, 0)
	if err != nil {
		b.Fatal(err)
	}
	var store Store
	if _, err := store.Apply(1, claim.Request(0).Command); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		record, exists := store.Get(claim.key)
		_, exists, err := ReadVirtualOwnership("counter", "one", RaftRuntimeRecord{Record: record, Exists: exists, Index: 1, Term: 1})
		if err != nil || !exists {
			b.Fatal(exists, err)
		}
	}
}
