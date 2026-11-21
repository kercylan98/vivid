package clusterstore

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/stretchr/testify/require"
)

func TestSnapshotTransfer_TCPAndWAL(t *testing.T) {
	model, _, _, _ := transferFixture(t, 5, SnapshotTransferLimits{})
	snapshot, err := model.nodes[1].Snapshot()
	require.NoError(t, err)
	source, err := CreateRaftStorage(filepath.Join(t.TempDir(), "source.wal"), RaftConfig{GroupID: "test", NodeID: 1, Voters: []uint64{1, 2, 3}})
	require.NoError(t, err)
	defer source.Close()
	require.NoError(t, source.Save(RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1}}))
	require.NoError(t, source.InstallSnapshot(snapshot))
	targetPath := filepath.Join(t.TempDir(), "target.wal")
	targetConfig := RaftConfig{GroupID: "test", NodeID: 3, Voters: []uint64{1, 2, 3}}
	target, err := CreateRaftStorage(targetPath, targetConfig)
	require.NoError(t, err)
	defer target.Close()
	model.nodes[1], err = NewRaftNode(source)
	require.NoError(t, err)
	model.nodes[3], err = NewRaftNode(target)
	require.NoError(t, err)
	drain := func(queue []RaftMessage, blocked bool) {
		for i := 0; len(queue) > 0; i++ {
			require.Less(t, i, 100)
			m := queue[0]
			queue = queue[1:]
			if blocked && (m.From == 3 || m.To == 3) {
				continue
			}
			out, err := model.nodes[m.To].Step(m)
			require.NoError(t, err)
			queue = append(queue, out...)
		}
	}
	out, err := model.nodes[1].Campaign()
	require.NoError(t, err)
	drain(out, true)
	out, err = model.nodes[1].Heartbeat()
	require.NoError(t, err)
	drain(out, false)
	budget, err := NewSnapshotTransferBudget(SnapshotTransferLimits{})
	require.NoError(t, err)
	sender, err := NewRaftSnapshotTransfer(model.nodes[1], budget)
	require.NoError(t, err)
	defer sender.Close()
	receiver, err := NewRaftSnapshotTransfer(model.nodes[3], budget)
	require.NoError(t, err)
	defer receiver.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	sendConn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	require.NoError(t, err)
	defer sendConn.Close()
	receiveConn, err := listener.Accept()
	require.NoError(t, err)
	defer receiveConn.Close()
	require.NoError(t, sendConn.SetDeadline(time.Now().Add(15*time.Second)))
	require.NoError(t, receiveConn.SetDeadline(time.Now().Add(15*time.Second)))
	writeRead := func(write, read net.Conn, frame *SnapshotFrame) SnapshotPacket {
		result := make(chan error, 1)
		go func() {
			_, err := remoting.NewDataFrame(frame.Bytes).WriteTo(write)
			frame.Release()
			result <- err
		}()
		// 与现有 Remoting 相同的 4 MiB 数据帧限制。
		wire, readErr := remoting.ReadFrame(read, nil, remoting.FrameReadLimits{MaxControlLen: 4 << 10, MaxDataLen: 4 << 20})
		require.NoError(t, <-result)
		require.NoError(t, readErr)
		require.Equal(t, remoting.FrameCtrlData, wire.Type)
		packet, err := DecodeSnapshotPacket(wire.Data)
		require.NoError(t, err)
		return packet
	}
	frame, err := sender.Send(3, time.Now())
	require.NoError(t, err)
	for i := 0; frame != nil; i++ {
		require.Less(t, i, 30)
		packet := writeRead(sendConn, receiveConn, frame)
		ack, _, err := receiver.Step(packet, time.Now())
		require.NoError(t, err)
		response := writeRead(receiveConn, sendConn, ack)
		frame, out, err = sender.Step(response, time.Now())
		require.NoError(t, err)
		drain(out, false)
	}
	require.EqualValues(t, 7, model.nodes[3].hard.Commit)
	require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
	receiver.Close()
	require.NoError(t, target.Close())
	restored, err := OpenRaftStorage(targetPath, targetConfig)
	require.NoError(t, err)
	defer restored.Close()
	actual, err := restored.Snapshot()
	require.NoError(t, err)
	require.Equal(t, snapshot, actual)
	node, err := NewRaftNode(restored)
	require.NoError(t, err)
	entries, err := node.Committed(actual.Index)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.EqualValues(t, 7, entries[0].Index)
	require.Empty(t, entries[0].Data)
}
