package clusterstore

import (
	"strconv"
	"testing"
)

// BenchmarkSnapshotPacket_EncodeDecode 仅测块编码、校验和解码，不含网络、重组或快照安装。
func BenchmarkSnapshotPacket_EncodeDecode(b *testing.B) {
	for _, size := range []int{64 << 10, SnapshotChunkBytes} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			packet := SnapshotPacket{Type: SnapshotChunk, ID: SnapshotTransferID{Session: [16]byte{1}, From: 1, To: 2, Term: 1, Request: 1, Index: 1, LogTerm: 1, Size: uint64(size)}, Data: make([]byte, size)}
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				data, err := EncodeSnapshotPacket(packet)
				if err != nil {
					b.Fatal(err)
				}
				decoded, err := DecodeSnapshotPacket(data)
				if err != nil || decoded.ID != packet.ID || len(decoded.Data) != size {
					b.Fatal("块编码或解码失败", err)
				}
			}
		})
	}
}
