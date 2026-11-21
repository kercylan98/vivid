package clusterstore

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type countedSnapshotStorage struct {
	raftNodeStorage
	reads int
}

func (s *countedSnapshotStorage) Snapshot() (RaftSnapshot, error) {
	s.reads++
	return s.raftNodeStorage.Snapshot()
}

func TestSnapshotTransfer_ReserveBeforeLoad(t *testing.T) {
	_, sender, _, budget := transferFixture(t, 1, SnapshotTransferLimits{MaxSnapshotBytes: 1})
	storage := &countedSnapshotStorage{raftNodeStorage: sender.node.storage}
	sender.node.storage = storage
	_, err := sender.Send(3, time.Now())
	require.ErrorIs(t, err, ErrSnapshotTransferCapacity)
	require.Zero(t, storage.reads, "额度不足时不能先加载整份快照")
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
}

func TestSnapshotTransfer_AckBeforeWriteCompletion(t *testing.T) {
	_, sender, receiver, budget := transferFixture(t, 1, SnapshotTransferLimits{})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	ack, _, err := receiver.Step(decodeTransferFrame(t, frame), now)
	require.NoError(t, err)
	_, _, err = sender.Step(decodeTransferFrame(t, ack), now)
	require.ErrorIs(t, err, ErrSnapshotTransferBusy)
	ack.Release()
	frame.Release()
	frame, err = sender.Send(3, now)
	require.NoError(t, err)
	require.EqualValues(t, SnapshotChunkBytes, decodeTransferFrame(t, frame).Offset)
	frame.Release()
	sender.Close()
	receiver.Close()
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
}

func TestSnapshotTransfer_InvalidIdentity(t *testing.T) {
	_, sender, receiver, budget := transferFixture(t, 0, SnapshotTransferLimits{})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	defer frame.Release()
	valid := decodeTransferFrame(t, frame)
	for _, mutate := range []func(*SnapshotPacket){
		func(p *SnapshotPacket) { p.ID.Cluster[0] ^= 1 },
		func(p *SnapshotPacket) { p.ID.From = 99 },
		func(p *SnapshotPacket) { p.ID.To = 2 },
		func(p *SnapshotPacket) { p.ID.LogTerm = p.ID.Term + 1 },
		func(p *SnapshotPacket) { p.ID.Size = ^uint64(0) },
		func(p *SnapshotPacket) { p.ID.Session = [16]byte{} },
		func(p *SnapshotPacket) { p.Type = 99 },
	} {
		packet := valid
		packet.ID.Term = 8
		mutate(&packet)
		_, _, err := receiver.Step(packet, now)
		require.ErrorIs(t, err, ErrInvalidSnapshotPacket)
		require.EqualValues(t, 1, receiver.node.hard.Term)
		require.Nil(t, receiver.incoming)
	}
	require.Equal(t, 1, budget.Usage().Sessions)
}

func TestSnapshotTransfer_WholeDigest(t *testing.T) {
	_, sender, receiver, budget := transferFixture(t, 0, SnapshotTransferLimits{})
	now := time.Now()
	frame, err := sender.Send(3, now)
	require.NoError(t, err)
	packet := decodeTransferFrame(t, frame)
	packet.Data = bytes.Clone(packet.Data)
	packet.Data[len(packet.Data)-1] ^= 1
	_, _, err = receiver.Step(packet, now)
	require.ErrorIs(t, err, ErrInvalidSnapshotPacket)
	frame.Release()
	require.EqualValues(t, 1, receiver.node.hard.Commit)
	require.Nil(t, receiver.incoming)
	sender.Close()
	receiver.Close()
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
}

func TestSnapshotTransfer_StorageFailure(t *testing.T) {
	for _, stage := range []string{"snapshot", "candidate", "closed", "published", "installed"} {
		t.Run(stage, func(t *testing.T) {
			_, sender, _, budget := transferFixture(t, 0, SnapshotTransferLimits{})
			disk, err := CreateRaftStorage(filepath.Join(t.TempDir(), "receive.wal"), RaftConfig{GroupID: "test", NodeID: 3, Voters: []uint64{1, 2, 3}})
			require.NoError(t, err)
			defer disk.Close()
			failure := errors.New("snapshot install interrupted")
			node, err := newRaftNode(&failingSnapshotStorage{RaftStorage: disk, stage: stage, err: failure})
			require.NoError(t, err)
			receiver, err := NewRaftSnapshotTransfer(node, budget)
			require.NoError(t, err)
			defer receiver.Close()
			frame, err := sender.Send(3, time.Now())
			require.NoError(t, err)
			ack, _, err := receiver.Step(decodeTransferFrame(t, frame), time.Now())
			require.ErrorIs(t, err, failure)
			require.Nil(t, ack, "安装失败不能发出 Done")
			frame.Release()
			require.Nil(t, receiver.incoming)
			require.False(t, receiver.completed)
			sender.Close()
			require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
		})
	}
}

func TestSnapshotTransfer_Config(t *testing.T) {
	for _, limits := range []SnapshotTransferLimits{
		{MaxSessions: -1}, {Timeout: -1}, {MaxInFlightBytes: 1},
	} {
		budget, err := NewSnapshotTransferBudget(limits)
		require.ErrorIs(t, err, ErrSnapshotTransferCapacity)
		require.Nil(t, budget)
	}
	_, sender, _, _ := transferFixture(t, 0, SnapshotTransferLimits{})
	_, err := sender.Send(3, time.Time{})
	require.ErrorIs(t, err, ErrInvalidSnapshotPacket)
}
