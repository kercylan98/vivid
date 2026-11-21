package clusterstore

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRaftNode_DuplicateVotes(t *testing.T) {
	network := newRaftNetwork(t, 5)
	out, err := network.nodes[1].Campaign()
	require.NoError(t, err)
	vote, err := network.nodes[2].Step(out[0])
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		out, err = network.nodes[1].Step(vote[0])
		require.NoError(t, err)
		require.Empty(t, out)
		require.Equal(t, RaftCandidate, network.nodes[1].role)
	}
	vote[0].From = 3
	_, err = network.nodes[1].Step(vote[0])
	require.NoError(t, err)
	require.Equal(t, RaftLeader, network.nodes[1].role)
}

func TestRaftNode_HigherTermResponse(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	leader := network.nodes[1]
	m := network.nodes[2].message(RaftAppendResponse, 1, 999, 0, 0)
	m.Term = 9
	m.Reject = true
	out, err := leader.Step(m)
	require.NoError(t, err)
	require.Empty(t, out)
	require.Equal(t, RaftFollower, leader.role, "高任期迟到应答仍须退让")
	require.EqualValues(t, 9, network.disks[1].state.hard.Term)
	require.Zero(t, network.disks[1].state.hard.Vote)
	_, _, err = leader.Propose([]byte("no"))
	require.ErrorIs(t, err, ErrRaftNotLeader)
}

func TestRaftNode_AppendResponseBounds(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	leader := network.nodes[1]
	_, out, err := leader.Propose([]byte("next"))
	require.NoError(t, err)
	request := out[0]
	ack, err := network.nodes[2].Step(request)
	require.NoError(t, err)
	invalid := ack[0]
	invalid.Match++
	_, err = leader.Step(invalid)
	require.ErrorIs(t, err, ErrInvalidRaftMessage)
	require.EqualValues(t, 1, leader.hard.Commit)
	require.Equal(t, request.Request, leader.peers[2].request)
	out, err = leader.Step(ack[0])
	require.NoError(t, err)
	require.EqualValues(t, 2, leader.hard.Commit)
	pending := leader.peers[2].request
	stale := ack[0]
	stale.Reject, stale.Match, stale.Hint = true, 0, 1
	ignored, err := leader.Step(stale)
	require.NoError(t, err)
	require.Empty(t, ignored)
	require.Equal(t, pending, leader.peers[2].request)
	require.EqualValues(t, 2, leader.peers[2].match)
	network.queue = out
	network.drain(t)
}

func TestRaftNode_StaleAppendCommit(t *testing.T) {
	network := newRaftNetwork(t, 3)
	disk := network.disks[2]
	require.NoError(t, disk.Save(RaftUpdate{HardState: RaftHardState{Term: 3, Commit: 1}, Entries: []RaftEntry{
		{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 2},
	}}))
	node, err := newRaftNode(disk)
	require.NoError(t, err)
	m := network.nodes[1].message(RaftAppendRequest, 2, 1, 1, 1)
	m.Term, m.Commit = 3, 3
	out, err := node.Step(m)
	require.NoError(t, err)
	require.False(t, out[0].Reject)
	require.EqualValues(t, 1, out[0].Match)
	require.EqualValues(t, 1, node.hard.Commit, "未匹配的旧后缀不能因心跳被提交")
	last, err := disk.LastIndex()
	require.NoError(t, err)
	require.EqualValues(t, 3, last, "短批次不能直接删除旧后缀")
	// 旧任期投票请求的拒绝应答仍须符合投票消息格式。
	m = network.nodes[1].message(RaftVoteRequest, 2, 0, 1, 1)
	m.Term = 1
	out, err = node.Step(m)
	require.NoError(t, err)
	require.True(t, network.nodes[1].validMessage(out[0]))
}

func TestRaftNode_SnapshotBoundary(t *testing.T) {
	network := newRaftNetwork(t, 3)
	// 以已压缩状态重启；空白节点不能通过普通复制得到基准以前的数据。
	for _, id := range []uint64{1, 3} {
		network.disks[id].state = raftPersistentState{baseIndex: 80, baseTerm: 2, hard: RaftHardState{Term: 2, Commit: 80}}
		var err error
		network.nodes[id], err = newRaftNode(network.disks[id])
		require.NoError(t, err)
	}
	network.campaign(t, 1)
	network.drain(t)
	leader := network.nodes[1]
	require.EqualValues(t, 81, leader.hard.Commit)
	peers, err := leader.Peers()
	require.NoError(t, err)
	require.Equal(t, []RaftPeerStatus{{ID: 2, SnapshotRequired: true}, {ID: 3, Match: 81}}, peers)
	_, err = leader.Committed(79)
	require.ErrorIs(t, err, ErrRaftCompacted)
	entries, err := leader.Committed(80)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.EqualValues(t, 81, entries[0].Index)
	status, err := leader.Status()
	require.NoError(t, err)
	require.EqualValues(t, 80, status.BaseIndex)
}

func TestRaftNode_Ownership(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	leader := network.nodes[1]
	_, out, err := leader.Propose([]byte("value"))
	require.NoError(t, err)
	// 修改一份出站消息不能污染存储、另一个接收方或后续重发。
	out[0].Entries[0].Data[0] = 'x'
	require.Equal(t, []byte("value"), out[1].Entries[0].Data)
	retry, err := leader.Heartbeat()
	require.NoError(t, err)
	require.Equal(t, []byte("value"), retry[0].Entries[0].Data)
	reply, err := network.nodes[2].Step(retry[0])
	require.NoError(t, err)
	clear(retry[0].Entries[0].Data)
	entry, err := network.disks[2].Entry(2)
	require.NoError(t, err)
	require.Equal(t, []byte("value"), entry.Data, "入站数据不能被存储借用")
	out, err = leader.Step(reply[0])
	require.NoError(t, err)
	network.queue = out
	network.drain(t)
	entries, err := leader.Committed(1)
	require.NoError(t, err)
	clear(entries[0].Data)
	entries, err = leader.Committed(1)
	require.NoError(t, err)
	require.Equal(t, []byte("value"), entries[0].Data)
}

func TestRaftNode_InvalidAppend(t *testing.T) {
	network := newRaftNetwork(t, 3)
	m := network.nodes[1].message(RaftAppendRequest, 2, 1, 0, 0)
	m.Term = 5
	for _, mutate := range []func(*RaftMessage){
		func(m *RaftMessage) { m.Request = 0 },
		func(m *RaftMessage) { m.Index = 1 },
		func(m *RaftMessage) { m.LogTerm = 1 },
		func(m *RaftMessage) { m.Reject = true },
		func(m *RaftMessage) { m.Match = 1 },
		func(m *RaftMessage) { m.Entries = []RaftEntry{{Index: 2, Term: 1}} },
		func(m *RaftMessage) { m.Entries = []RaftEntry{{Index: 1}} },
		func(m *RaftMessage) { m.Entries = []RaftEntry{{Index: 1, Term: 6}} },
		func(m *RaftMessage) { m.Entries = []RaftEntry{{Index: 1, Term: 3}, {Index: 2, Term: 2}} },
		func(m *RaftMessage) {
			m.Entries = []RaftEntry{{Index: 1, Term: 1, Data: make([]byte, maxRaftEntryBytes+1)}}
		},
		func(m *RaftMessage) { m.Entries = make([]RaftEntry, maxRaftMessageEntries+1) },
		func(m *RaftMessage) {
			m.Entries = []RaftEntry{{Index: 1, Term: 1, Data: make([]byte, maxRaftEntryBytes)}, {Index: 2, Term: 1, Data: make([]byte, maxRaftEntryBytes)}}
		},
	} {
		invalid := m
		mutate(&invalid)
		out, err := network.nodes[2].Step(invalid)
		require.ErrorIs(t, err, ErrInvalidRaftMessage)
		require.Empty(t, out)
		require.Zero(t, network.nodes[2].hard.Term)
	}
}

func TestRaftNode_Exhaustion(t *testing.T) {
	network := newRaftNetwork(t, 3)
	last := ^uint64(0)
	for id, disk := range network.disks {
		disk.state = raftPersistentState{baseIndex: last - 1, baseTerm: 1, hard: RaftHardState{Term: 1, Commit: last - 1}}
		var err error
		network.nodes[id], err = newRaftNode(disk)
		require.NoError(t, err)
	}
	// 这里的压缩基准不适用普通网络检查，从同步消息队列直接送达。
	out, err := network.nodes[1].Campaign()
	require.NoError(t, err)
	for i := 0; len(out) > 0; i++ {
		require.Less(t, i, 50)
		m := out[0]
		out = out[1:]
		next, err := network.nodes[m.To].Step(m)
		require.NoError(t, err)
		out = append(out, next...)
	}
	leader := network.nodes[1]
	require.Equal(t, last, leader.hard.Commit)
	out, err = leader.Heartbeat()
	require.NoError(t, err)
	require.Equal(t, last, out[0].Index)
	require.Empty(t, out[0].Entries)
	_, _, err = leader.Propose([]byte("full"))
	require.ErrorIs(t, err, ErrRaftExhausted)
	entries, err := leader.Committed(last - 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	leader.sequence = last
	// 消费当前心跳后，下一次新请求不能复用溢出的序号。
	reply, err := network.nodes[2].Step(out[0])
	require.NoError(t, err)
	_, err = leader.Step(reply[0])
	require.NoError(t, err)
	_, err = leader.Heartbeat()
	require.ErrorIs(t, err, ErrRaftExhausted)
	_, err = leader.Status()
	require.ErrorIs(t, err, ErrRaftNodeFailed)
	network.disks[2].state.hard.Term = last
	node, err := newRaftNode(network.disks[2])
	require.NoError(t, err)
	_, err = node.Campaign()
	require.ErrorIs(t, err, ErrRaftExhausted)
}

func TestRaftNode_DiskFailure(t *testing.T) {
	for _, stage := range []string{"campaign", "vote", "append", "commit"} {
		for _, mode := range []string{"short", "sync"} {
			t.Run(stage+"/"+mode, func(t *testing.T) {
				disk, path := createRaftStorage(t)
				require.NoError(t, disk.Save(RaftUpdate{HardState: RaftHardState{Term: 1}}))
				node, err := NewRaftNode(disk)
				require.NoError(t, err)
				m := RaftMessage{Cluster: node.cluster, From: 2, To: 1, Term: 1, Type: RaftVoteRequest}
				if stage == "append" {
					m.Type, m.Request = RaftAppendRequest, 1
					m.Entries = []RaftEntry{{Index: 1, Term: 1, Data: []byte("payload")}}
				}
				if stage == "commit" {
					_, err = node.Campaign()
					require.NoError(t, err)
					m.Type, m.Term = RaftVoteResponse, 2
					out, err := node.Step(m)
					require.NoError(t, err)
					m.Type, m.Request, m.Index, m.Match = RaftAppendResponse, out[0].Request, out[0].Index, 1
				}
				fault := &failingJournalFile{journalFile: disk.journal.file}
				disk.journal.file = fault
				failure := errors.New("sync failed")
				expected := error(failure)
				if mode == "short" {
					fault.writeLimit, expected = journalRecordHeader+1, io.ErrShortWrite
				} else {
					fault.syncErr = failure
				}
				before := node.hard
				var out []RaftMessage
				if stage == "campaign" {
					out, err = node.Campaign()
				} else {
					out, err = node.Step(m)
				}
				require.ErrorIs(t, err, expected)
				require.Empty(t, out, "未刷盘成功不能发送投票或复制确认")
				require.Equal(t, before, node.hard)
				_, err = node.Heartbeat()
				require.ErrorIs(t, err, ErrRaftNodeFailed)
				require.NoError(t, disk.Close())
				restored, err := OpenRaftStorage(path, raftTestConfig())
				require.NoError(t, err)
				defer restored.Close()
				if stage == "vote" && mode == "sync" {
					node, err = NewRaftNode(restored)
					require.NoError(t, err)
					m.From = 3
					out, err = node.Step(m)
					require.NoError(t, err)
					require.True(t, out[0].Reject, "完整但未确认的投票恢复后仍不可改投")
				}
			})
		}
	}
}

func TestRaftNode_CapacityRejection(t *testing.T) {
	network := newRaftNetwork(t, 1)
	network.campaign(t, 1)
	node := network.nodes[1]
	network.disks[1].failure = ErrRaftCapacity
	_, out, err := node.Propose(bytes.Repeat([]byte{'a'}, 64))
	require.ErrorIs(t, err, ErrRaftCapacity)
	require.Empty(t, out)
	network.disks[1].failure = nil
	index, _, err := node.Propose([]byte("retry"))
	require.NoError(t, err)
	require.EqualValues(t, 2, index, "预检容量拒绝不破坏现有节点")
}
