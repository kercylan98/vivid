package clusterstore

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotPacket_Codec(t *testing.T) {
	id := SnapshotTransferID{Session: [16]byte{1}, From: 1, To: 2, Term: 1, Request: 1, Index: 1, LogTerm: 1, Size: 3}
	for _, p := range []SnapshotPacket{
		{ID: id, Type: SnapshotChunk, Data: []byte("abc")},
		{ID: id, Type: SnapshotChunkAck, ReceiverTerm: 1, Offset: 3, Done: true},
		{ID: id, Type: SnapshotChunkAck, ReceiverTerm: 2, Reject: true},
	} {
		data, err := EncodeSnapshotPacket(p)
		require.NoError(t, err)
		decoded, err := DecodeSnapshotPacket(data)
		require.NoError(t, err)
		require.Equal(t, p.ID, decoded.ID)
		require.Equal(t, p.Type, decoded.Type)
		require.Equal(t, p.Done, decoded.Done)
		require.Equal(t, p.Reject, decoded.Reject)
		require.Equal(t, p.Offset, decoded.Offset)
		require.Equal(t, p.ReceiverTerm, decoded.ReceiverTerm)
		require.True(t, bytes.Equal(p.Data, decoded.Data))
		for size := 0; size < len(data); size++ {
			_, err := DecodeSnapshotPacket(data[:size])
			require.ErrorIs(t, err, ErrInvalidSnapshotPacket)
		}
		for position := range data {
			corrupt := bytes.Clone(data)
			corrupt[position] ^= 1
			_, err := DecodeSnapshotPacket(corrupt)
			require.ErrorIs(t, err, ErrInvalidSnapshotPacket, "包头和负载任一字节损坏都应拒绝")
		}
	}
}

func FuzzSnapshotPacket_Decode(f *testing.F) {
	id := SnapshotTransferID{Session: [16]byte{1}, From: 1, To: 2, Term: 1, Request: 1, Index: 1, LogTerm: 1, Size: 3}
	data, err := EncodeSnapshotPacket(SnapshotPacket{ID: id, Type: SnapshotChunk, Data: []byte("abc")})
	require.NoError(f, err)
	f.Add(data)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		packet, err := DecodeSnapshotPacket(data)
		if err != nil {
			require.ErrorIs(t, err, ErrInvalidSnapshotPacket)
			return
		}
		encoded, err := EncodeSnapshotPacket(packet)
		require.NoError(t, err)
		require.Equal(t, data, encoded)
		require.LessOrEqual(t, len(encoded), snapshotPacketHeaderBytes+SnapshotChunkBytes)
	})
}
