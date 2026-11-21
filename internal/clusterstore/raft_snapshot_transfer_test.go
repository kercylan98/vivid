package clusterstore

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func transferFixture(t testing.TB, records int, limits SnapshotTransferLimits) (*raftNetwork, *RaftSnapshotTransfer, *RaftSnapshotTransfer, *SnapshotTransferBudget) {
	t.Helper()
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	network.blocked[3] = true
	var store Store
	_, err := store.Apply(1, Command{})
	require.NoError(t, err)
	if records == 0 {
		index, out, err := network.nodes[1].Propose([]byte("entry"))
		require.NoError(t, err)
		network.queue = out
		network.drain(t)
		_, err = store.Apply(index, Command{})
		require.NoError(t, err)
	}
	for i := range records {
		index, out, err := network.nodes[1].Propose([]byte("entry"))
		require.NoError(t, err)
		network.queue = out
		network.drain(t)
		_, err = store.Apply(index, Command{Operation: Put, Key: fmt.Sprint(i), Value: bytes.Repeat([]byte{byte(i)}, maxValueBytes)})
		require.NoError(t, err)
	}
	require.NoError(t, network.nodes[1].Compact(store.Snapshot()))
	budget, err := NewSnapshotTransferBudget(limits)
	require.NoError(t, err)
	sender, err := NewRaftSnapshotTransfer(network.nodes[1], budget)
	require.NoError(t, err)
	receiver, err := NewRaftSnapshotTransfer(network.nodes[3], budget)
	require.NoError(t, err)
	t.Cleanup(sender.Close)
	t.Cleanup(receiver.Close)
	return network, sender, receiver, budget
}

func decodeTransferFrame(t testing.TB, frame *SnapshotFrame) SnapshotPacket {
	t.Helper()
	require.NotNil(t, frame)
	packet, err := DecodeSnapshotPacket(frame.Bytes)
	require.NoError(t, err)
	require.LessOrEqual(t, len(frame.Bytes), SnapshotChunkBytes+snapshotPacketHeaderBytes)
	return packet
}

func TestSnapshotTransfer_LargeRoundTrip(t *testing.T) {
	network, sender, receiver, budget := transferFixture(t, 5, SnapshotTransferLimits{})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	for count := 0; frame != nil; count++ {
		require.Less(t, count, 30)
		packet := decodeTransferFrame(t, frame)
		require.Greater(t, packet.ID.Size, uint64(4<<20))
		ack, messages, err := receiver.Step(packet, now)
		require.NoError(t, err)
		require.Empty(t, messages)
		frame.Release()
		response := decodeTransferFrame(t, ack)
		if !response.Done {
			require.EqualValues(t, 1, network.nodes[3].hard.Commit)
			require.EqualValues(t, 1, network.nodes[1].peers[3].match)
		}
		frame, messages, err = sender.Step(response, now)
		require.NoError(t, err)
		ack.Release()
		network.queue = append(network.queue, messages...)
	}
	clear(network.blocked)
	network.drain(t)
	require.EqualValues(t, 6, network.nodes[3].hard.Commit)
	snapshot, err := network.nodes[3].Snapshot()
	require.NoError(t, err)
	store, err := Restore(snapshot.Data)
	require.NoError(t, err)
	for i := range 5 {
		record, ok := store.Get(fmt.Sprint(i))
		require.True(t, ok)
		require.Equal(t, bytes.Repeat([]byte{byte(i)}, maxValueBytes), record.Value)
	}
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
}

func TestSnapshotTransfer_LostFinalAck(t *testing.T) {
	_, sender, receiver, budget := transferFixture(t, 0, SnapshotTransferLimits{})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	packet := decodeTransferFrame(t, frame)
	ack, _, err := receiver.Step(packet, now)
	require.NoError(t, err)
	require.True(t, decodeTransferFrame(t, ack).Done)
	ack.Release()
	frame.Release()
	before := receiver.node.hard
	frame, err = sender.Send(3, now)
	require.NoError(t, err)
	ack, _, err = receiver.Step(decodeTransferFrame(t, frame), now)
	require.NoError(t, err)
	frame.Release()
	require.Equal(t, before, receiver.node.hard)
	_, _, err = sender.Step(decodeTransferFrame(t, ack), now)
	require.NoError(t, err)
	ack.Release()
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
}

func TestSnapshotTransfer_ReorderAndCorruption(t *testing.T) {
	_, sender, receiver, budget := transferFixture(t, 1, SnapshotTransferLimits{})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	defer frame.Release()
	first := decodeTransferFrame(t, frame)
	ack, _, err := receiver.Step(first, now)
	require.NoError(t, err)
	ack.Release()
	deadline := receiver.incoming.deadline
	// 乱序块不填补空洞，也不延长进度截止时间。
	future := first
	future.Offset = 2 * SnapshotChunkBytes
	future.Data = bytes.Clone(sender.outgoing[3].data[future.Offset : future.Offset+SnapshotChunkBytes])
	ack, _, err = receiver.Step(future, now.Add(time.Second))
	require.NoError(t, err)
	require.EqualValues(t, SnapshotChunkBytes, decodeTransferFrame(t, ack).Offset)
	ack.Release()
	require.Equal(t, deadline, receiver.incoming.deadline)
	ack, _, err = receiver.Step(first, now.Add(time.Second))
	require.NoError(t, err)
	ack.Release()
	require.Equal(t, deadline, receiver.incoming.deadline)
	corrupt := first
	corrupt.Data = bytes.Clone(first.Data)
	corrupt.Data[0] ^= 1
	_, _, err = receiver.Step(corrupt, now)
	require.ErrorIs(t, err, ErrInvalidSnapshotPacket)
	require.Nil(t, receiver.incoming)
	require.EqualValues(t, 1, receiver.node.hard.Commit)
	require.Equal(t, 1, budget.Usage().Sessions)
}

func TestSnapshotTransfer_QuotaAndFrameLifetime(t *testing.T) {
	_, sender, receiver, budget := transferFixture(t, 1, SnapshotTransferLimits{MaxSessions: 1})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	packet := decodeTransferFrame(t, frame)
	_, _, err = receiver.Step(packet, now)
	require.ErrorIs(t, err, ErrSnapshotTransferCapacity)
	require.Nil(t, receiver.incoming)
	sender.Cancel(3)
	require.Zero(t, budget.Usage().Sessions)
	require.Positive(t, budget.Usage().InFlightBytes)
	_, err = sender.Send(3, now)
	require.ErrorIs(t, err, ErrSnapshotTransferBusy, "取消不能越过尚未归还的同副本发送缓冲")
	sender.Close()
	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() { defer wait.Done(); frame.Release() }()
	}
	wait.Wait()
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
	_, err = sender.Send(3, now)
	require.ErrorIs(t, err, ErrSnapshotTransferClosed)
}

func TestSnapshotTransfer_FrameBackpressure(t *testing.T) {
	_, sender, receiver, budget := transferFixture(t, 1, SnapshotTransferLimits{MaxInFlightBytes: SnapshotChunkBytes + snapshotPacketHeaderBytes})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	packet := decodeTransferFrame(t, frame)
	packet.Data = bytes.Clone(packet.Data)
	_, _, err = receiver.Step(packet, now)
	require.ErrorIs(t, err, ErrSnapshotTransferCapacity)
	require.EqualValues(t, SnapshotChunkBytes, receiver.incoming.offset)
	frame.Release()
	ack, _, err := receiver.Step(packet, now)
	require.NoError(t, err)
	response := decodeTransferFrame(t, ack)
	// 确认仍在写出时不能占满下一整块，释放后可继续生成。
	_, _, err = sender.Step(response, now)
	require.ErrorIs(t, err, ErrSnapshotTransferCapacity)
	ack.Release()
	frame, err = sender.Send(3, now)
	require.NoError(t, err)
	require.EqualValues(t, SnapshotChunkBytes, decodeTransferFrame(t, frame).Offset)
	frame.Release()
	sender.Close()
	receiver.Close()
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
}

func TestSnapshotTransfer_ExpireAndTerm(t *testing.T) {
	network, sender, receiver, budget := transferFixture(t, 1, SnapshotTransferLimits{Timeout: time.Second})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	packet := decodeTransferFrame(t, frame)
	ack, _, err := receiver.Step(packet, now)
	require.NoError(t, err)
	ack.Release()
	frame.Release()
	require.NoError(t, receiver.Expire(now.Add(time.Second)))
	require.Nil(t, receiver.incoming)
	frame, err = sender.Send(3, now)
	require.NoError(t, err)
	ack, _, err = receiver.Step(decodeTransferFrame(t, frame), now.Add(time.Second))
	require.NoError(t, err)
	require.True(t, decodeTransferFrame(t, ack).Reject)
	frame.Release()
	_, _, err = sender.Step(decodeTransferFrame(t, ack), now)
	require.ErrorIs(t, err, ErrSnapshotTransferStale)
	ack.Release()
	frame, err = sender.Send(3, now.Add(time.Second))
	require.NoError(t, err)
	packet = decodeTransferFrame(t, frame)
	// 更高任期必须先持久化，并释放旧接收会话。
	_, err = network.nodes[3].Campaign()
	require.NoError(t, err)
	ack, _, err = receiver.Step(packet, now.Add(time.Second))
	require.NoError(t, err)
	frame.Release()
	response := decodeTransferFrame(t, ack)
	require.EqualValues(t, 2, response.ReceiverTerm)
	_, _, err = sender.Step(response, now.Add(time.Second))
	require.NoError(t, err)
	ack.Release()
	require.Equal(t, RaftFollower, network.nodes[1].role)
	require.EqualValues(t, 2, network.disks[1].state.hard.Term)
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
}

func TestSnapshotTransfer_NewerSnapshot(t *testing.T) {
	network, sender, receiver, budget := transferFixture(t, 1, SnapshotTransferLimits{})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	old := decodeTransferFrame(t, frame)
	old.Data = bytes.Clone(old.Data)
	frame.Release()
	_, out, err := network.nodes[1].Propose([]byte("new"))
	require.NoError(t, err)
	network.queue = out
	network.drain(t)
	require.NoError(t, network.nodes[1].Compact(remoteSnapshot(3, 1).Data))
	frame, err = sender.Send(3, now)
	require.NoError(t, err)
	current := decodeTransferFrame(t, frame)
	require.Greater(t, current.ID.Request, old.ID.Request)
	ack, _, err := receiver.Step(current, now)
	require.NoError(t, err)
	frame.Release()
	_, _, err = sender.Step(decodeTransferFrame(t, ack), now)
	require.NoError(t, err)
	ack.Release()
	ack, _, err = receiver.Step(old, now)
	require.NoError(t, err)
	require.True(t, decodeTransferFrame(t, ack).Reject)
	ack.Release()
	require.EqualValues(t, 3, receiver.node.hard.Commit)
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
}
