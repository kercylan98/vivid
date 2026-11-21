package clusterstore

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func snapshotNetwork(t testing.TB) *raftNetwork {
	t.Helper()
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	network.blocked[3] = true
	for range 3 {
		_, out, err := network.nodes[1].Propose([]byte("entry"))
		require.NoError(t, err)
		network.queue = out
		network.drain(t)
	}
	require.NoError(t, network.nodes[1].Compact(remoteSnapshot(3, 1).Data))
	out, err := network.nodes[1].Heartbeat()
	require.NoError(t, err)
	network.queue = out
	network.drain(t)
	return network
}

func TestRaftNode_SnapshotCatchup(t *testing.T) {
	network := snapshotNetwork(t)
	leader := network.nodes[1]
	out, err := leader.SendSnapshot(3)
	require.NoError(t, err)
	require.EqualValues(t, 3, out[0].Snapshot.Index)
	request := out[0]
	// 普通心跳不能作废快照请求，也不会自动加载大型快照。
	heartbeat, err := leader.Heartbeat()
	require.NoError(t, err)
	for _, m := range heartbeat {
		require.NotEqualValues(t, 3, m.To)
	}
	retry, err := leader.SendSnapshot(3)
	require.NoError(t, err)
	require.Equal(t, request.Request, retry[0].Request)
	clear(request.Snapshot.Data)
	require.NotEqual(t, request.Snapshot.Data, retry[0].Snapshot.Data)
	clear(network.blocked)
	// 丢失安装确认后再次安装，只应回复，不能重写或回退状态。
	ack, err := network.nodes[3].Step(retry[0])
	require.NoError(t, err)
	require.EqualValues(t, 3, network.nodes[3].hard.Commit)
	before := network.disks[3].snapshot
	again, err := network.nodes[3].Step(retry[0])
	require.NoError(t, err)
	require.Equal(t, ack, again)
	require.Equal(t, before, network.disks[3].snapshot)
	out, err = leader.Step(again[0])
	require.NoError(t, err)
	network.queue = out
	network.drain(t)
	for _, node := range network.nodes {
		require.EqualValues(t, 4, node.hard.Commit)
	}
	peers, err := leader.Peers()
	require.NoError(t, err)
	require.Equal(t, RaftPeerStatus{ID: 3, Match: 4}, peers[1])
	_, err = network.nodes[3].Committed(0)
	require.ErrorIs(t, err, ErrRaftCompacted)
	snapshot, err := network.nodes[3].Snapshot()
	require.NoError(t, err)
	require.EqualValues(t, 3, snapshot.Index)
	entries, err := network.nodes[3].Committed(snapshot.Index)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.EqualValues(t, 4, entries[0].Index)
	_, err = leader.SendSnapshot(3)
	require.ErrorIs(t, err, ErrInvalidRaftUpdate)
	_, err = network.nodes[2].SendSnapshot(3)
	require.ErrorIs(t, err, ErrRaftNotLeader)
}

func TestRaftNode_SnapshotReplacement(t *testing.T) {
	network := snapshotNetwork(t)
	leader := network.nodes[1]
	old, err := leader.SendSnapshot(3)
	require.NoError(t, err)
	require.NoError(t, leader.Compact(remoteSnapshot(4, 1).Data))
	next, err := leader.SendSnapshot(3)
	require.NoError(t, err)
	require.NotEqual(t, old[0].Request, next[0].Request)
	ack, err := network.nodes[3].Step(old[0])
	require.NoError(t, err)
	out, err := leader.Step(ack[0])
	require.NoError(t, err)
	require.Empty(t, out)
	require.True(t, leader.peers[3].snapshot)
	require.Equal(t, next[0].Request, leader.peers[3].request)
	ack, err = network.nodes[3].Step(next[0])
	require.NoError(t, err)
	out, err = leader.Step(ack[0])
	require.NoError(t, err)
	clear(network.blocked)
	network.queue = out
	network.drain(t)
	// 新快照已经安装后，迟到旧请求不能回退基准或应用数据。
	_, err = network.nodes[3].Step(old[0])
	require.NoError(t, err)
	snapshot, err := network.nodes[3].Snapshot()
	require.NoError(t, err)
	require.EqualValues(t, 4, snapshot.Index)
	require.EqualValues(t, 4, network.nodes[3].hard.Commit)
}

func TestRaftNode_SnapshotValidation(t *testing.T) {
	network := snapshotNetwork(t)
	out, err := network.nodes[1].SendSnapshot(3)
	require.NoError(t, err)
	valid := out[0]
	for _, mutate := range []func(*RaftMessage){
		func(m *RaftMessage) { m.Request = 0 },
		func(m *RaftMessage) { m.Index++ },
		func(m *RaftMessage) { m.LogTerm++ },
		func(m *RaftMessage) { m.Snapshot = nil },
		func(m *RaftMessage) { m.Snapshot.Data = []byte("bad") },
		func(m *RaftMessage) { m.Snapshot.Data = remoteSnapshot(4, 1).Data },
		func(m *RaftMessage) { m.Cluster[0] ^= 1 },
		func(m *RaftMessage) { m.Reject = true },
		func(m *RaftMessage) { m.Commit = 1 },
		func(m *RaftMessage) { m.Entries = []RaftEntry{{Index: 1, Term: 1}} },
	} {
		m := valid
		snapshot := *valid.Snapshot
		m.Snapshot = &snapshot
		m.Term = 9
		mutate(&m)
		_, err := network.nodes[3].Step(m)
		require.ErrorIs(t, err, ErrInvalidRaftMessage)
		require.EqualValues(t, 1, network.nodes[3].hard.Term, "无效快照不能改变任期")
	}
	// 高任期快照确认也须让旧 Leader 退让，即使本次请求已经迟到。
	m := network.nodes[3].message(RaftSnapshotResponse, 1, valid.Request+1, 3, 0)
	m.Term, m.Match = 9, 3
	_, err = network.nodes[1].Step(m)
	require.NoError(t, err)
	require.Equal(t, RaftFollower, network.nodes[1].role)
	_, err = network.nodes[1].Step(valid)
	require.ErrorIs(t, err, ErrInvalidRaftMessage, "请求接收者必须匹配")
	valid.To = 1
	valid.From = 3
	response, err := network.nodes[1].Step(valid)
	require.NoError(t, err)
	require.True(t, response[0].Reject)
	require.Equal(t, RaftSnapshotResponse, response[0].Type)
	require.True(t, network.nodes[3].validMessage(response[0]))
}

type failingSnapshotStorage struct {
	*RaftStorage
	stage string
	err   error
}

func (s *failingSnapshotStorage) InstallSnapshot(snapshot RaftSnapshot) error {
	return s.RaftStorage.installSnapshot(snapshot, func(stage string) error {
		if stage == s.stage {
			return s.err
		}
		return nil
	})
}

func TestRaftNode_SnapshotDiskFailure(t *testing.T) {
	for _, stage := range []string{"snapshot", "candidate", "closed", "published", "installed"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "node.wal")
			storage := installFixture(t, path)
			failure := errors.New("snapshot I/O failed")
			node, err := newRaftNode(&failingSnapshotStorage{RaftStorage: storage, stage: stage, err: failure})
			require.NoError(t, err)
			snapshot := remoteSnapshot(8, 3)
			m := RaftMessage{Cluster: node.cluster, Type: RaftSnapshotRequest, From: 2, To: 1, Term: 4, Request: 1, Index: 8, LogTerm: 3, Snapshot: &snapshot}
			out, err := node.Step(m)
			require.ErrorIs(t, err, failure)
			require.Empty(t, out)
			_, err = node.Status()
			require.ErrorIs(t, err, ErrRaftNodeFailed)
			require.NoError(t, storage.Close())
			restored, err := OpenRaftStorage(path, raftTestConfig())
			require.NoError(t, err)
			defer restored.Close()
			node, err = NewRaftNode(restored)
			require.NoError(t, err)
			out, err = node.Step(m)
			require.NoError(t, err, "恢复后允许重发，完整安装过的请求直接确认")
			require.EqualValues(t, 8, out[0].Match)
			require.EqualValues(t, 8, node.hard.Commit)
		})
	}
}

func TestRaftNode_SnapshotCommittedPrefix(t *testing.T) {
	storage := installFixture(t, filepath.Join(t.TempDir(), "prefix.wal"))
	defer storage.Close()
	node, err := NewRaftNode(storage)
	require.NoError(t, err)
	snapshot := remoteSnapshot(1, 2)
	m := RaftMessage{Cluster: node.cluster, Type: RaftSnapshotRequest, From: 2, To: 1, Term: 4, Request: 1, Index: 1, LogTerm: 2, Snapshot: &snapshot}
	out, err := node.Step(m)
	require.ErrorIs(t, err, ErrInvalidRaftUpdate)
	require.Empty(t, out)
	require.Zero(t, storage.state.baseIndex)
	require.EqualValues(t, 1, storage.state.hard.Commit)
	entry, err := storage.Entry(1)
	require.NoError(t, err)
	require.Equal(t, []byte("committed"), entry.Data)
}

func TestRaftNode_SnapshotMaximumIndex(t *testing.T) {
	network := newRaftNetwork(t, 3)
	last := ^uint64(0)
	for _, id := range []uint64{1, 2} {
		network.disks[id].state = raftPersistentState{baseIndex: last - 1, baseTerm: 1, hard: RaftHardState{Term: 1, Commit: last - 1}}
		network.disks[id].snapshot = remoteSnapshot(last-1, 1)
		var err error
		network.nodes[id], err = newRaftNode(network.disks[id])
		require.NoError(t, err)
	}
	network.blocked[3] = true
	network.campaign(t, 1)
	network.drain(t)
	require.Equal(t, last, network.nodes[1].hard.Commit)
	require.NoError(t, network.nodes[1].Compact(remoteSnapshot(last, 2).Data))
	out, err := network.nodes[1].SendSnapshot(3)
	require.NoError(t, err)
	clear(network.blocked)
	network.queue = out
	network.drain(t)
	require.Equal(t, last, network.nodes[3].hard.Commit)
	require.Equal(t, last, network.nodes[1].peers[3].match)
	out, err = network.nodes[1].Heartbeat()
	require.NoError(t, err)
	network.queue = out
	network.drain(t)
}

func TestRaftNode_SnapshotTermSyncFailure(t *testing.T) {
	storage := installFixture(t, filepath.Join(t.TempDir(), "term.wal"))
	defer storage.Close()
	node, err := NewRaftNode(storage)
	require.NoError(t, err)
	failure := errors.New("term sync failed")
	storage.journal.file = &failingJournalFile{journalFile: storage.journal.file, syncErr: failure}
	snapshot := remoteSnapshot(8, 3)
	m := RaftMessage{Cluster: node.cluster, Type: RaftSnapshotRequest, From: 2, To: 1, Term: 5, Request: 1, Index: 8, LogTerm: 3, Snapshot: &snapshot}
	out, err := node.Step(m)
	require.ErrorIs(t, err, failure)
	require.Empty(t, out)
	require.EqualValues(t, 4, node.hard.Term)
	require.Zero(t, storage.state.baseIndex)
	_, err = node.Snapshot()
	require.ErrorIs(t, err, ErrRaftNodeFailed)
}

func TestRaftNode_SnapshotBusinessRecovery(t *testing.T) {
	nodes := make(map[uint64]*RaftNode)
	disks := make(map[uint64]*RaftStorage)
	stores := make(map[uint64]*Store)
	configs := make(map[uint64]RaftConfig)
	paths := make(map[uint64]string)
	for id := uint64(1); id <= 3; id++ {
		configs[id] = RaftConfig{GroupID: "business", NodeID: id, Voters: []uint64{1, 2, 3}}
		paths[id] = filepath.Join(t.TempDir(), "business.wal")
		disk, err := CreateRaftStorage(paths[id], configs[id])
		require.NoError(t, err)
		t.Cleanup(func() { disk.Close() })
		disks[id] = disk
		nodes[id], err = NewRaftNode(disk)
		require.NoError(t, err)
		stores[id] = &Store{}
	}
	blocked := false
	apply := func() {
		for id, node := range nodes {
			for {
				entries, err := node.Committed(stores[id].Index())
				if errors.Is(err, ErrRaftCompacted) {
					snapshot, err := node.Snapshot()
					require.NoError(t, err)
					stores[id], err = Restore(snapshot.Data)
					require.NoError(t, err)
					continue
				}
				require.NoError(t, err)
				if len(entries) == 0 {
					break
				}
				for _, entry := range entries {
					var command Command
					if len(entry.Data) > 0 {
						require.NoError(t, json.Unmarshal(entry.Data, &command))
					}
					result, err := stores[id].Apply(entry.Index, command)
					require.NoError(t, err)
					require.True(t, result.Applied)
				}
			}
		}
	}
	drain := func(queue []RaftMessage) {
		for i := 0; len(queue) > 0; i++ {
			require.Less(t, i, 100)
			m := queue[0]
			queue = queue[1:]
			if blocked && (m.From == 3 || m.To == 3) {
				continue
			}
			out, err := nodes[m.To].Step(m)
			require.NoError(t, err)
			queue = append(queue, out...)
		}
		apply()
	}
	out, err := nodes[1].Campaign()
	require.NoError(t, err)
	drain(out)
	blocked = true
	commands := []Command{
		{Operation: Put, Key: "orders", Value: []byte("a")},
		{Operation: Put, Key: "orders", ExpectedRevision: 2, Value: []byte("b")},
		{Operation: Delete, Key: "orders", ExpectedRevision: 3},
		{Operation: Put, Key: "orders", Value: []byte("c")},
	}
	for i, command := range commands {
		if i == 3 {
			require.NoError(t, nodes[1].Compact(stores[1].Snapshot()))
		}
		data, err := json.Marshal(command)
		require.NoError(t, err)
		_, out, err := nodes[1].Propose(data)
		require.NoError(t, err)
		drain(out)
	}
	out, err = nodes[1].SendSnapshot(3)
	require.NoError(t, err)
	blocked = false
	drain(out)
	for _, store := range stores {
		require.Equal(t, stores[1].Snapshot(), store.Snapshot())
		record, ok := store.Get("orders")
		require.True(t, ok)
		require.Equal(t, Record{Value: []byte("c"), Revision: 5}, record)
	}
	// 接收快照后重启，从快照恢复再重放后缀，版本与其余副本保持一致。
	require.NoError(t, disks[3].Close())
	restored, err := OpenRaftStorage(paths[3], configs[3])
	require.NoError(t, err)
	defer restored.Close()
	nodes[3], err = NewRaftNode(restored)
	require.NoError(t, err)
	stores[3] = &Store{}
	apply()
	require.Equal(t, stores[1].Snapshot(), stores[3].Snapshot())
}
