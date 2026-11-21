package clusterstore

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/stretchr/testify/require"
)

func TestRaftPacket_RoundTripAndDamage(t *testing.T) {
	for _, message := range []RaftMessage{
		{Type: RaftVoteRequest, From: 1, To: 2, Term: 2, Index: 1, LogTerm: 1},
		{Type: RaftVoteResponse, From: 2, To: 1, Term: 2, Reject: true},
		{Type: RaftAppendRequest, From: 1, To: 2, Term: 2, Request: 1, Entries: []RaftEntry{{Index: 1, Term: 2, Data: []byte("command")}}},
		{Type: RaftAppendResponse, From: 2, To: 1, Term: 2, Request: 1, Match: 1},
	} {
		data, err := EncodeRaftPacket(message)
		require.NoError(t, err)
		actual, err := DecodeRaftPacket(data)
		require.NoError(t, err)
		require.Equal(t, message, actual)
		for size := range len(data) {
			_, err := DecodeRaftPacket(data[:size])
			require.ErrorIs(t, err, ErrInvalidRaftMessage)
		}
		for offset := range data {
			data[offset] ^= 1
			_, err := DecodeRaftPacket(data)
			require.ErrorIs(t, err, ErrInvalidRaftMessage)
			data[offset] ^= 1
		}
	}
}

func TestRaftPacket_FrameBudget(t *testing.T) {
	message := RaftMessage{Type: RaftAppendRequest, From: 1, To: 2, Term: 1, Request: 1, Entries: []RaftEntry{
		{Index: 1, Term: 1, Data: make([]byte, maxRaftEntryBytes)},
		{Index: 2, Term: 1, Data: make([]byte, maxRaftPacketBytes-raftPacketHeaderBytes-2*raftEntryHeaderBytes-maxRaftEntryBytes)},
	}}
	data, err := EncodeRaftPacket(message)
	require.NoError(t, err)
	require.Len(t, data, maxRaftPacketBytes)
	writer := serialization.NewWriter().Write(false, strings.Repeat("a", maxRaftRouteBytes), strings.Repeat("b", maxRaftRouteBytes))
	codec := serialization.NewVividCodec(nil)
	require.NoError(t, codec.RegisterMessage("SYS", "*clusterstore.RaftWire", new(RaftWire)))
	require.NoError(t, codec.Encode(writer, &RaftWire{Data: data}))
	frame := remoting.NewDataFrame(writer.Bytes())
	_, err = remoting.ReadFrame(bytes.NewReader(frame.Bytes()), nil, remoting.FrameReadLimits{MaxDataLen: 4 << 20})
	require.NoError(t, err, "最大协议包加两个最长引用仍须通过现有网络帧限制")
	message.Entries[1].Data = append(message.Entries[1].Data, 1)
	_, err = EncodeRaftPacket(message)
	require.ErrorIs(t, err, ErrInvalidRaftMessage)

	// 原 WAL 上限允许的批次未必能装入网络包，发送时必须拆分且保持重发身份。
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	for i := 2; i <= 3; i++ {
		_, _, err := network.nodes[1].Propose(make([]byte, maxRaftEntryBytes-1024))
		require.NoError(t, err)
	}
	peer := network.nodes[1].peers[2]
	peer.request, peer.next = 0, 2
	out, err := network.nodes[1].sendAppend(2)
	require.NoError(t, err)
	require.Len(t, out[0].Entries, 1)
	_, err = EncodeRaftPacket(out[0])
	require.NoError(t, err)
	retry, err := network.nodes[1].sendAppend(2)
	require.NoError(t, err)
	require.Equal(t, out, retry)
}

func TestRaftPacket_InvalidStructure(t *testing.T) {
	valid := RaftMessage{Type: RaftAppendRequest, From: 1, To: 2, Term: 1, Request: 1}
	for _, mutate := range []func([]byte){
		func(data []byte) { data[8] = byte(RaftSnapshotResponse) },
		func(data []byte) { data[9] = 2 },
		func(data []byte) { binary.LittleEndian.PutUint64(data[48:], 0) },
		func(data []byte) { binary.LittleEndian.PutUint32(data[120:], maxRaftMessageEntries+1) },
	} {
		data, err := EncodeRaftPacket(valid)
		require.NoError(t, err)
		mutate(data)
		binary.LittleEndian.PutUint32(data[12:], packetChecksum(data))
		_, err = DecodeRaftPacket(data)
		require.ErrorIs(t, err, ErrInvalidRaftMessage)
	}
	_, err := EncodeRaftPacket(RaftMessage{Type: RaftSnapshotResponse, From: 1, To: 2, Term: 1, Request: 1, Index: 1, Match: 1})
	require.ErrorIs(t, err, ErrInvalidRaftMessage, "不能从普通网络包绕过分块最终确认")
}

func FuzzRaftPacket_Decode(f *testing.F) {
	seed, _ := EncodeRaftPacket(RaftMessage{Type: RaftVoteRequest, From: 1, To: 2, Term: 1})
	f.Add(seed)
	f.Add([]byte("VIVIDRM1"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 16<<10 {
			return
		}
		message, err := DecodeRaftPacket(data)
		if err == nil {
			encoded, err := EncodeRaftPacket(message)
			require.NoError(t, err)
			require.Equal(t, data, encoded)
		}
	})
}

func BenchmarkRaftPacket_EncodeDecode(b *testing.B) {
	message := RaftMessage{Type: RaftAppendRequest, From: 1, To: 2, Term: 1, Request: 1, Entries: []RaftEntry{{Index: 1, Term: 1, Data: make([]byte, 64)}}}
	b.ReportAllocs()
	for b.Loop() {
		data, err := EncodeRaftPacket(message)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := DecodeRaftPacket(data); err != nil {
			b.Fatal(err)
		}
	}
}
