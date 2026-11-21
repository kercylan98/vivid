package clusterstore

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// memoryRaftStorage 使用同一状态约束，省略磁盘延迟，以便大量重排协议消息。
// 真实刷盘顺序另由磁盘存储和故障注入用例验证。
type memoryRaftStorage struct {
	config   RaftConfig
	state    raftPersistentState
	failure  error
	saves    int
	snapshot RaftSnapshot
}

func (s *memoryRaftStorage) Configuration() RaftConfig     { return s.config }
func (s *memoryRaftStorage) State() (RaftHardState, error) { return s.state.hard, nil }
func (s *memoryRaftStorage) LastIndex() (uint64, error)    { return s.state.lastIndex(), nil }
func (s *memoryRaftStorage) LogBase() (uint64, uint64, error) {
	return s.state.baseIndex, s.state.baseTerm, nil
}
func (s *memoryRaftStorage) Term(index uint64) (uint64, error) {
	if index < s.state.baseIndex {
		return 0, ErrRaftCompacted
	}
	if index == s.state.baseIndex {
		return s.state.baseTerm, nil
	}
	if index > s.state.lastIndex() {
		return 0, ErrRaftEntryMissing
	}
	return s.state.entries[index-s.state.baseIndex-1].Term, nil
}
func (s *memoryRaftStorage) Entry(index uint64) (RaftEntry, error) {
	if index <= s.state.baseIndex {
		return RaftEntry{}, ErrRaftCompacted
	}
	if index > s.state.lastIndex() {
		return RaftEntry{}, ErrRaftEntryMissing
	}
	entry := s.state.entries[index-s.state.baseIndex-1]
	entry.Data = bytes.Clone(entry.Data)
	return entry, nil
}
func (s *memoryRaftStorage) Save(update RaftUpdate) error {
	if s.failure != nil {
		return s.failure
	}
	storage := RaftStorage{config: s.config, state: s.state}
	change, err := storage.prepare(update)
	if err != nil {
		return err
	}
	storage.state.install(change)
	s.state = storage.state
	s.saves++
	return nil
}

func (s *memoryRaftStorage) Snapshot() (RaftSnapshot, error) {
	snapshot := s.snapshot
	snapshot.Data = bytes.Clone(snapshot.Data)
	return snapshot, nil
}

func (s *memoryRaftStorage) SnapshotMetadata() (RaftSnapshotMetadata, error) {
	return RaftSnapshotMetadata{Index: s.snapshot.Index, Term: s.snapshot.Term, Size: uint64(len(s.snapshot.Data)), Digest: sha256.Sum256(s.snapshot.Data)}, nil
}

func (s *memoryRaftStorage) Compact(data []byte) error {
	if s.failure != nil {
		return s.failure
	}
	store, err := Restore(data)
	if err != nil {
		return err
	}
	index := store.Index()
	if index <= s.state.baseIndex || index > s.state.hard.Commit {
		return ErrInvalidRaftUpdate
	}
	term, err := s.Term(index)
	if err != nil {
		return err
	}
	s.installSnapshot(RaftSnapshot{Index: index, Term: term, Data: data}, s.state.hard, s.state.entries[index-s.state.baseIndex:])
	return nil
}

func (s *memoryRaftStorage) InstallSnapshot(snapshot RaftSnapshot) error {
	if s.failure != nil {
		return s.failure
	}
	storage := RaftStorage{config: s.config, state: s.state}
	// prepareSnapshot 仅查询状态，真实存储的可用性检查由磁盘用例覆盖。
	hard, suffix, err := storage.prepareSnapshot(snapshot)
	if err != nil {
		return err
	}
	s.installSnapshot(snapshot, hard, suffix)
	return nil
}

func (s *memoryRaftStorage) installSnapshot(snapshot RaftSnapshot, hard RaftHardState, suffix []RaftEntry) {
	entries := append([]RaftEntry(nil), suffix...)
	s.state = raftPersistentState{baseIndex: snapshot.Index, baseTerm: snapshot.Term, hard: hard, entries: entries}
	for _, entry := range entries {
		s.state.bytes += len(entry.Data)
	}
	snapshot.Data = bytes.Clone(snapshot.Data)
	s.snapshot = snapshot
}

type raftNetwork struct {
	nodes     map[uint64]*RaftNode
	disks     map[uint64]*memoryRaftStorage
	queue     []RaftMessage
	blocked   map[uint64]bool
	leaders   map[uint64]uint64
	committed map[uint64]RaftEntry
}

func newRaftNetwork(t testing.TB, count int) *raftNetwork {
	t.Helper()
	voters := make([]uint64, count)
	for i := range voters {
		voters[i] = uint64(i + 1)
	}
	network := &raftNetwork{nodes: make(map[uint64]*RaftNode), disks: make(map[uint64]*memoryRaftStorage), blocked: make(map[uint64]bool), leaders: make(map[uint64]uint64), committed: make(map[uint64]RaftEntry)}
	for _, id := range voters {
		config, err := normalizeRaftConfig(RaftConfig{GroupID: "test", NodeID: id, Voters: voters})
		require.NoError(t, err)
		disk := &memoryRaftStorage{config: config}
		node, err := newRaftNode(disk)
		require.NoError(t, err)
		network.nodes[id], network.disks[id] = node, disk
	}
	return network
}

func (s *raftNetwork) campaign(t testing.TB, id uint64) {
	t.Helper()
	messages, err := s.nodes[id].Campaign()
	require.NoError(t, err)
	s.queue = append(s.queue, messages...)
	s.check(t)
}

func (s *raftNetwork) deliver(t testing.TB, index int) {
	t.Helper()
	m := s.queue[index]
	s.queue = append(s.queue[:index], s.queue[index+1:]...)
	if s.blocked[m.From] || s.blocked[m.To] {
		return
	}
	out, err := s.nodes[m.To].Step(m)
	require.NoError(t, err, "message %+v", m)
	s.queue = append(s.queue, out...)
	s.check(t)
}

func (s *raftNetwork) drain(t testing.TB) {
	t.Helper()
	for i := 0; len(s.queue) > 0; i++ {
		require.Less(t, i, 10000, "消息不能无限应答循环")
		s.deliver(t, 0)
	}
}

func (s *raftNetwork) recover(t testing.TB) *RaftNode {
	t.Helper()
	clear(s.blocked)
	s.drain(t)
	// 停止故障后让最新日志持有者重试，直到它获知已有最高任期并完成竞选。
	for attempt := 0; attempt < len(s.nodes)+2; attempt++ {
		var candidate *RaftNode
		var lastIndex, lastTerm uint64
		for id := uint64(1); id <= uint64(len(s.nodes)); id++ {
			node := s.nodes[id]
			index, err := node.storage.LastIndex()
			require.NoError(t, err)
			term, err := node.storage.Term(index)
			require.NoError(t, err)
			if candidate == nil || term > lastTerm || term == lastTerm && index > lastIndex {
				candidate, lastIndex, lastTerm = node, index, term
			}
		}
		var out []RaftMessage
		var err error
		if candidate.role == RaftLeader {
			out, err = candidate.Heartbeat()
		} else {
			out, err = candidate.Campaign()
		}
		require.NoError(t, err)
		s.queue = append(s.queue, out...)
		s.drain(t)
		if candidate.role == RaftLeader {
			for _, id := range candidate.config.Voters {
				if peer := candidate.peers[id]; peer != nil && peer.snapshot {
					out, err := candidate.SendSnapshot(id)
					require.NoError(t, err)
					s.queue = append(s.queue, out...)
				}
			}
			s.drain(t)
			return candidate
		}
	}
	t.Fatal("故障停止后未能重新选出 Leader")
	return nil
}

func (s *raftNetwork) check(t testing.TB) {
	t.Helper()
	for id, node := range s.nodes {
		if node.role == RaftLeader {
			if other := s.leaders[node.hard.Term]; other != 0 {
				require.Equal(t, other, id, "同任期只能选出一个 Leader")
			}
			s.leaders[node.hard.Term] = id
		}
		state := s.disks[id].state
		require.Equal(t, state.hard, node.hard)
		for after := state.baseIndex; after < state.hard.Commit; after++ {
			index := after + 1
			entry, err := s.disks[id].Entry(index)
			require.NoError(t, err)
			if existing, ok := s.committed[index]; ok {
				require.Equal(t, existing, entry, "各节点已提交前缀必须一致")
			} else {
				s.committed[index] = entry
			}
		}
		for index, existing := range s.committed {
			if index > state.baseIndex && index <= state.hard.Commit {
				entry, err := s.disks[id].Entry(index)
				require.NoError(t, err)
				require.Equal(t, existing, entry)
			}
		}
	}
}

func TestRaftNode_ElectionReplication(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	require.Equal(t, RaftLeader, network.nodes[1].role)
	for _, node := range network.nodes {
		require.EqualValues(t, 1, node.hard.Commit)
	}
	data := []byte("orders")
	index, out, err := network.nodes[1].Propose(data)
	require.NoError(t, err)
	require.EqualValues(t, 2, index)
	data[0] = 'x'
	network.queue = append(network.queue, out...)
	network.drain(t)
	for _, node := range network.nodes {
		require.EqualValues(t, 2, node.hard.Commit)
		entries, err := node.Committed(0)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		require.Empty(t, entries[0].Data)
		require.Equal(t, []byte("orders"), entries[1].Data)
	}
	_, _, err = network.nodes[2].Propose([]byte("no"))
	require.ErrorIs(t, err, ErrRaftNotLeader)
	_, err = network.nodes[2].Heartbeat()
	require.ErrorIs(t, err, ErrRaftNotLeader)
	_, _, err = network.nodes[1].Propose(nil)
	require.ErrorIs(t, err, ErrInvalidRaftUpdate)
}

func TestRaftNode_PartitionRecovery(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	network.blocked[1] = true
	_, out, err := network.nodes[1].Propose([]byte("orphan"))
	require.NoError(t, err)
	network.queue = append(network.queue, out...)
	network.drain(t)
	require.EqualValues(t, 1, network.nodes[1].hard.Commit, "少数派本地写入不能提交")
	network.campaign(t, 2)
	network.drain(t)
	require.Equal(t, RaftLeader, network.nodes[2].role)
	_, out, err = network.nodes[2].Propose([]byte("majority"))
	require.NoError(t, err)
	network.queue = append(network.queue, out...)
	network.drain(t)
	require.EqualValues(t, 3, network.nodes[2].hard.Commit)
	clear(network.blocked)
	out, err = network.nodes[2].Heartbeat()
	require.NoError(t, err)
	network.queue = append(network.queue, out...)
	network.drain(t)
	for id, node := range network.nodes {
		require.EqualValues(t, 3, node.hard.Commit)
		entry, err := network.disks[id].Entry(2)
		require.NoError(t, err)
		require.Empty(t, entry.Data, "旧 Leader 未提交后缀被新任期空操作替换")
	}
	require.Equal(t, RaftFollower, network.nodes[1].role)
}

func TestRaftNode_DuplicateAndStaleResponse(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	request := network.queue[0]
	reply, err := network.nodes[2].Step(request)
	require.NoError(t, err)
	_, err = network.nodes[1].Step(reply[0])
	require.NoError(t, err)
	_, err = network.nodes[1].Step(reply[0])
	require.NoError(t, err)
	require.Equal(t, RaftLeader, network.nodes[1].role)
	first, err := network.nodes[1].Heartbeat()
	require.NoError(t, err)
	_, _, err = network.nodes[1].Propose([]byte("next"))
	require.NoError(t, err)
	second, err := network.nodes[1].Heartbeat()
	require.NoError(t, err)
	require.Equal(t, first[0].Request, second[0].Request, "重发不能作废仍在途的确认")
	current, err := network.nodes[2].Step(first[0])
	require.NoError(t, err)
	out, err := network.nodes[1].Step(current[0])
	require.NoError(t, err)
	require.EqualValues(t, 1, network.nodes[1].hard.Commit)
	pending := network.nodes[1].peers[2].request
	require.NotEqual(t, first[0].Request, pending)
	_, err = network.nodes[1].Step(current[0])
	require.NoError(t, err)
	require.EqualValues(t, 1, network.nodes[1].hard.Commit)
	require.Equal(t, pending, network.nodes[1].peers[2].request, "迟到确认不能完成新批次")
	network.queue = out
	network.drain(t)
	require.EqualValues(t, 2, network.nodes[1].hard.Commit)
}

func TestRaftNode_VoteRestartAndLogFreshness(t *testing.T) {
	network := newRaftNetwork(t, 3)
	candidate := network.nodes[1]
	requests, err := candidate.Campaign()
	require.NoError(t, err)
	reply, err := network.nodes[2].Step(requests[0])
	require.NoError(t, err)
	require.False(t, reply[0].Reject)
	network.nodes[2], err = newRaftNode(network.disks[2])
	require.NoError(t, err)
	request := requests[0]
	request.From = 3
	reply, err = network.nodes[2].Step(request)
	require.NoError(t, err)
	require.True(t, reply[0].Reject, "重启不能在同任期再次投票")
	// 先比较末尾任期，再比较位置。
	disk := network.disks[2]
	require.NoError(t, disk.Save(RaftUpdate{HardState: RaftHardState{Term: 3}, Entries: []RaftEntry{{Index: 1, Term: 2}}}))
	network.nodes[2], err = newRaftNode(disk)
	require.NoError(t, err)
	request.Term = 4
	request.Index = 100
	request.LogTerm = 1
	reply, err = network.nodes[2].Step(request)
	require.NoError(t, err)
	require.True(t, reply[0].Reject)
	request.Index = 1
	request.LogTerm = 3
	reply, err = network.nodes[2].Step(request)
	require.NoError(t, err)
	require.False(t, reply[0].Reject)
}

func TestRaftNode_PreviousTermCommitRule(t *testing.T) {
	network := newRaftNetwork(t, 3)
	entries := make([]RaftEntry, 70)
	for i := range entries {
		entries[i] = RaftEntry{Index: uint64(i + 1), Term: 1}
	}
	require.NoError(t, network.disks[1].Save(RaftUpdate{HardState: RaftHardState{Term: 1}, Entries: entries}))
	var err error
	network.nodes[1], err = newRaftNode(network.disks[1])
	require.NoError(t, err)
	leader := network.nodes[1]
	out, err := leader.Campaign()
	require.NoError(t, err)
	out, err = network.nodes[2].Step(out[0])
	require.NoError(t, err)
	out, err = leader.Step(out[0])
	require.NoError(t, err)
	// 空节点先拒绝前驱位置，再分批接收旧任期条目。
	out, err = network.nodes[2].Step(out[0])
	require.NoError(t, err)
	out, err = leader.Step(out[0])
	require.NoError(t, err)
	require.Len(t, out[0].Entries, maxRaftMessageEntries)
	out, err = network.nodes[2].Step(out[0])
	require.NoError(t, err)
	out, err = leader.Step(out[0])
	require.NoError(t, err)
	require.EqualValues(t, maxRaftMessageEntries, leader.peers[2].match)
	require.Zero(t, leader.hard.Commit)
	out, err = network.nodes[2].Step(out[0])
	require.NoError(t, err)
	_, err = leader.Step(out[0])
	require.NoError(t, err)
	require.EqualValues(t, 71, leader.hard.Commit)
}

func TestRaftNode_BoundedCatchup(t *testing.T) {
	network := newRaftNetwork(t, 3)
	disk := network.disks[1]
	entries := make([]RaftEntry, 150)
	for i := range entries {
		entries[i] = RaftEntry{Index: uint64(i + 1), Term: 1, Data: []byte("log")}
	}
	require.NoError(t, disk.Save(RaftUpdate{HardState: RaftHardState{Term: 1}, Entries: entries}))
	var err error
	network.nodes[1], err = newRaftNode(disk)
	require.NoError(t, err)
	network.campaign(t, 1)
	network.drain(t)
	for _, node := range network.nodes {
		require.EqualValues(t, 151, node.hard.Commit)
	}
	committed, err := network.nodes[1].Committed(0)
	require.NoError(t, err)
	require.Len(t, committed, maxRaftMessageEntries)
}

func TestRaftNode_SingleMember(t *testing.T) {
	network := newRaftNetwork(t, 1)
	network.campaign(t, 1)
	node := network.nodes[1]
	require.EqualValues(t, 1, node.hard.Commit)
	index, out, err := node.Propose([]byte("single"))
	require.NoError(t, err)
	require.Empty(t, out)
	require.Equal(t, index, node.hard.Commit)
	_, err = node.Campaign()
	require.ErrorIs(t, err, ErrInvalidRaftUpdate)
	_, err = node.Committed(index + 1)
	require.ErrorIs(t, err, ErrInvalidRaftUpdate)
}

func TestRaftNode_InvalidMessage(t *testing.T) {
	network := newRaftNetwork(t, 3)
	requests, err := network.nodes[1].Campaign()
	require.NoError(t, err)
	valid := requests[0]
	for _, mutate := range []func(*RaftMessage){
		func(m *RaftMessage) { m.Cluster[0] ^= 1 }, func(m *RaftMessage) { m.From = 99 }, func(m *RaftMessage) { m.From = m.To },
		func(m *RaftMessage) { m.To = 3 }, func(m *RaftMessage) { m.Term = 0 }, func(m *RaftMessage) { m.Type = 255 },
		func(m *RaftMessage) { m.LogTerm = m.Term + 1 }, func(m *RaftMessage) { m.Request = 1 }, func(m *RaftMessage) { m.Index = 1 },
	} {
		m := valid
		mutate(&m)
		_, err := network.nodes[2].Step(m)
		require.ErrorIs(t, err, ErrInvalidRaftMessage)
		require.Zero(t, network.nodes[2].hard.Term, "非法身份与结构在修改任期前拒绝")
	}
	// 相同组名、不同固定成员的消息也必须拒绝。
	other := newRaftNetwork(t, 5)
	requests, err = other.nodes[1].Campaign()
	require.NoError(t, err)
	_, err = network.nodes[2].Step(requests[0])
	require.ErrorIs(t, err, ErrInvalidRaftMessage)
}

func TestRaftNode_StorageFailure(t *testing.T) {
	network := newRaftNetwork(t, 3)
	requests, err := network.nodes[1].Campaign()
	require.NoError(t, err)
	failure := errors.New("sync failed")
	network.disks[2].failure = failure
	out, err := network.nodes[2].Step(requests[0])
	require.ErrorIs(t, err, failure)
	require.Empty(t, out)
	out, err = network.nodes[2].Campaign()
	require.ErrorIs(t, err, ErrRaftNodeFailed)
	require.Empty(t, out)
	_, err = network.nodes[2].Status()
	require.ErrorIs(t, err, ErrRaftNodeFailed)
}

func TestRaftNode_DiskRecovery(t *testing.T) {
	paths := make(map[uint64]string)
	nodes := make(map[uint64]*RaftNode)
	disks := make(map[uint64]*RaftStorage)
	for _, id := range []uint64{1, 2, 3} {
		path := filepath.Join(t.TempDir(), "node.wal")
		paths[id] = path
		disk, err := CreateRaftStorage(path, RaftConfig{GroupID: "disk", NodeID: id, Voters: []uint64{1, 2, 3}})
		require.NoError(t, err)
		disks[id] = disk
		node, err := NewRaftNode(disk)
		require.NoError(t, err)
		nodes[id] = node
		t.Cleanup(func() { disk.Close() })
	}
	drain := func(queue []RaftMessage) {
		for len(queue) > 0 {
			m := queue[0]
			queue = queue[1:]
			out, err := nodes[m.To].Step(m)
			require.NoError(t, err)
			queue = append(queue, out...)
		}
	}
	out, err := nodes[1].Campaign()
	require.NoError(t, err)
	drain(out)
	_, out, err = nodes[1].Propose([]byte("durable"))
	require.NoError(t, err)
	drain(out)
	for id, disk := range disks {
		require.NoError(t, disk.Close())
		restored, err := OpenRaftStorage(paths[id], RaftConfig{GroupID: "disk", NodeID: id, Voters: []uint64{1, 2, 3}})
		require.NoError(t, err)
		defer restored.Close()
		nodes[id], err = NewRaftNode(restored)
		require.NoError(t, err)
		status, err := nodes[id].Status()
		require.NoError(t, err)
		require.Equal(t, RaftFollower, status.Role)
		require.EqualValues(t, 2, status.Commit)
		entries, err := nodes[id].Committed(0)
		require.NoError(t, err)
		require.Equal(t, []byte("durable"), entries[1].Data)
	}
}

func TestRaftNode_RandomNetwork(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			network := newRaftNetwork(t, 5)
			for step := 0; step < 300; step++ {
				id := uint64(rng.Intn(5) + 1)
				node := network.nodes[id]
				switch rng.Intn(7) {
				case 0:
					if node.role != RaftLeader {
						network.campaign(t, id)
					}
				case 1:
					if node.role == RaftLeader {
						_, out, err := node.Propose([]byte(fmt.Sprintf("%d/%d", seed, step)))
						require.NoError(t, err)
						network.queue = append(network.queue, out...)
					}
				case 2:
					if node.role == RaftLeader {
						out, err := node.Heartbeat()
						require.NoError(t, err)
						network.queue = append(network.queue, out...)
					}
				case 3:
					network.blocked[id] = !network.blocked[id]
				case 4:
					var err error
					network.nodes[id], err = newRaftNode(network.disks[id])
					require.NoError(t, err)
				case 5:
					if len(network.queue) > 0 && len(network.queue) < 500 {
						network.queue = append(network.queue, network.queue[rng.Intn(len(network.queue))])
					}
				case 6:
					if len(network.queue) > 0 {
						index := rng.Intn(len(network.queue))
						network.queue = append(network.queue[:index], network.queue[index+1:]...)
					}
				}
				if len(network.queue) > 0 {
					network.deliver(t, rng.Intn(len(network.queue)))
				}
				network.check(t)
			}
			leader := network.recover(t)
			index, out, err := leader.Propose([]byte("recovered"))
			require.NoError(t, err)
			network.queue = append(network.queue, out...)
			network.drain(t)
			for _, node := range network.nodes {
				require.Equal(t, index, node.hard.Commit)
			}
		})
	}
}
