package clusterstore

import (
	"crypto/sha256"
	"errors"
	"sort"
)

const maxRaftMessageEntries = 64

var (
	ErrRaftNotLeader      = errors.New("raft node is not the leader")
	ErrInvalidRaftMessage = errors.New("invalid raft message")
	ErrRaftExhausted      = errors.New("raft term, index or request sequence exhausted")
	ErrRaftNodeFailed     = errors.New("raft node is unavailable after fatal failure")
)

type RaftRole uint8

const (
	RaftFollower RaftRole = iota
	RaftCandidate
	RaftLeader
)

type RaftMessageType uint8

const (
	RaftVoteRequest RaftMessageType = iota + 1
	RaftVoteResponse
	RaftAppendRequest
	RaftAppendResponse
	RaftSnapshotRequest
	RaftSnapshotResponse
)

// RaftMessage 的 Cluster 绑定组名与固定投票成员，不提供认证。
// Append 的 Index/LogTerm 表示前一条位置/任期；Vote 表示候选者末尾位置/任期。
// 响应回显 Request；Append 成功时 Match 为本次匹配末尾，失败时 Hint 为重试位置。
// Snapshot 请求的 Index/LogTerm 与快照末尾一致，确认只表示已持久化，不代表业务已应用。
type RaftMessage struct {
	Cluster                 [sha256.Size]byte
	Type                    RaftMessageType
	Reject                  bool
	From, To, Term, Request uint64
	Index, LogTerm, Commit  uint64
	Entries                 []RaftEntry
	Snapshot                *RaftSnapshot
	Match, Hint             uint64
}

// RaftStatus 的 Leader 是本地所知信息，不保证当前仍有多数派或可执行线性一致读。
type RaftStatus struct {
	ID, Term, Leader, Commit, LastIndex, BaseIndex uint64
	Role                                           RaftRole
}

// RaftPeerStatus 暴露复制进度；SnapshotRequired 表示普通日志复制已无法补齐。
type RaftPeerStatus struct {
	ID, Match        uint64
	SnapshotRequired bool
}

type raftNodeStorage interface {
	Configuration() RaftConfig
	State() (RaftHardState, error)
	LastIndex() (uint64, error)
	LogBase() (uint64, uint64, error)
	Term(uint64) (uint64, error)
	Entry(uint64) (RaftEntry, error)
	Save(RaftUpdate) error
	Snapshot() (RaftSnapshot, error)
	Compact([]byte) error
	InstallSnapshot(RaftSnapshot) error
}

type raftPeer struct {
	next, match             uint64
	request, previous, sent uint64
	snapshot                bool
}

// RaftNode 是固定成员的串行共识核心，无后台任务、时钟或网络，不直接执行业务。
// 独占 storage，调用方不得并行访问或修改它；出站消息归调用方所有，错误时不发送消息。
// 调用方负责竞选退避、心跳周期及传输，不能将本地 Propose 成功视为已经提交。
type RaftNode struct {
	storage  raftNodeStorage
	config   RaftConfig
	cluster  [sha256.Size]byte
	hard     RaftHardState
	role     RaftRole
	leader   uint64
	sequence uint64
	votes    map[uint64]bool
	peers    map[uint64]*raftPeer
	failed   error
}

func NewRaftNode(storage *RaftStorage) (*RaftNode, error) {
	if storage == nil {
		return nil, ErrJournalClosed
	}
	return newRaftNode(storage)
}

func newRaftNode(storage raftNodeStorage) (*RaftNode, error) {
	config, err := normalizeRaftConfig(storage.Configuration())
	if err != nil {
		return nil, err
	}
	hard, err := storage.State()
	if err != nil {
		return nil, err
	}
	identity := config
	identity.NodeID = 0 // 相同成员组在不同节点上使用同一消息边界
	return &RaftNode{storage: storage, config: config, hard: hard, cluster: sha256.Sum256(encodeRaftConfig(identity)), peers: make(map[uint64]*raftPeer)}, nil
}

func (n *RaftNode) Status() (RaftStatus, error) {
	if err := n.available(); err != nil {
		return RaftStatus{}, err
	}
	last, err := n.storage.LastIndex()
	if err != nil {
		return RaftStatus{}, n.fail(err)
	}
	base, _, err := n.storage.LogBase()
	if err != nil {
		return RaftStatus{}, n.fail(err)
	}
	return RaftStatus{ID: n.config.NodeID, Term: n.hard.Term, Leader: n.leader, Commit: n.hard.Commit, LastIndex: last, BaseIndex: base, Role: n.role}, nil
}

func (n *RaftNode) Peers() ([]RaftPeerStatus, error) {
	if err := n.available(); err != nil {
		return nil, err
	}
	var result []RaftPeerStatus
	for _, id := range n.config.Voters {
		if peer := n.peers[id]; peer != nil {
			result = append(result, RaftPeerStatus{ID: id, Match: peer.match, SnapshotRequired: peer.snapshot})
		}
	}
	return result, nil
}

// Campaign 由上层在选举超时后调用，自投票刷盘后才发送请求；Follower 重启从未当选状态开始。
func (n *RaftNode) Campaign() ([]RaftMessage, error) {
	if err := n.available(); err != nil {
		return nil, err
	}
	if n.role == RaftLeader {
		return nil, ErrInvalidRaftUpdate
	}
	if n.hard.Term == ^uint64(0) {
		return nil, ErrRaftExhausted
	}
	hard := n.hard
	hard.Term++
	hard.Vote = n.config.NodeID
	if err := n.save(hard, nil); err != nil {
		return nil, err
	}
	n.role, n.leader = RaftCandidate, 0
	n.votes = map[uint64]bool{n.config.NodeID: true}
	if len(n.config.Voters) == 1 {
		return n.becomeLeader()
	}
	last, err := n.storage.LastIndex()
	if err != nil {
		return nil, n.fail(err)
	}
	term, err := n.storage.Term(last)
	if err != nil {
		return nil, n.fail(err)
	}
	var messages []RaftMessage
	for _, id := range n.config.Voters {
		if id != n.config.NodeID {
			messages = append(messages, n.message(RaftVoteRequest, id, 0, last, term))
		}
	}
	return messages, nil
}

// Heartbeat 发送有界日志批次或重试未确认批次，不累计内部发送队列。
func (n *RaftNode) Heartbeat() ([]RaftMessage, error) {
	if err := n.available(); err != nil {
		return nil, err
	}
	if n.role != RaftLeader {
		return nil, ErrRaftNotLeader
	}
	return n.broadcast()
}

// Propose 保存一个非空负载并开始复制，返回分配的位置；空负载保留给选举空操作。
// 返回成功只保证本地保存，调用方应以 Commit 和已应用结果确认业务完成。
func (n *RaftNode) Propose(data []byte) (uint64, []RaftMessage, error) {
	if err := n.available(); err != nil {
		return 0, nil, err
	}
	if n.role != RaftLeader {
		return 0, nil, ErrRaftNotLeader
	}
	if len(data) == 0 || len(data) > maxRaftEntryBytes {
		return 0, nil, ErrInvalidRaftUpdate
	}
	last, err := n.storage.LastIndex()
	if err != nil {
		return 0, nil, n.fail(err)
	}
	if last == ^uint64(0) {
		return 0, nil, ErrRaftExhausted
	}
	index := last + 1
	if err := n.save(n.hard, []RaftEntry{{Index: index, Term: n.hard.Term, Data: data}}); err != nil {
		return 0, nil, err
	}
	if err := n.advanceCommit(); err != nil {
		return 0, nil, err
	}
	messages, err := n.broadcast()
	return index, messages, err
}

// Step 处理一个完整消息，所有必要刷盘完成后才返回应答；拒绝不属于固定组的消息。
func (n *RaftNode) Step(m RaftMessage) ([]RaftMessage, error) {
	if err := n.available(); err != nil {
		return nil, err
	}
	if !n.validMessage(m) {
		return nil, ErrInvalidRaftMessage
	}
	if m.Term > n.hard.Term {
		hard := n.hard
		hard.Term, hard.Vote = m.Term, 0
		if err := n.save(hard, nil); err != nil {
			return nil, err
		}
		n.follow(0)
	}
	if m.Term < n.hard.Term {
		if m.Type == RaftVoteRequest || m.Type == RaftAppendRequest || m.Type == RaftSnapshotRequest {
			typeID := RaftVoteResponse
			index := uint64(0)
			if m.Type == RaftAppendRequest {
				typeID = RaftAppendResponse
				index = m.Index
			}
			if m.Type == RaftSnapshotRequest {
				typeID, index = RaftSnapshotResponse, m.Index
			}
			reply := n.message(typeID, m.From, m.Request, index, 0)
			reply.Reject = true
			return []RaftMessage{reply}, nil
		}
		return nil, nil
	}
	switch m.Type {
	case RaftVoteRequest:
		return n.vote(m)
	case RaftVoteResponse:
		if n.role != RaftCandidate || m.Reject {
			return nil, nil
		}
		n.votes[m.From] = true
		if len(n.votes) > len(n.config.Voters)/2 {
			return n.becomeLeader()
		}
	case RaftAppendRequest:
		return n.appendRequest(m)
	case RaftAppendResponse:
		return n.appendResponse(m)
	case RaftSnapshotRequest:
		return n.snapshotRequest(m)
	case RaftSnapshotResponse:
		return n.snapshotResponse(m)
	}
	return nil, nil
}

// Committed 返回 after 之后的有界已提交日志副本，调用方在成功应用后自行推进位置。
// 空负载也必须推进应用位置；若 after 落后于压缩基准，先恢复快照。
func (n *RaftNode) Committed(after uint64) ([]RaftEntry, error) {
	if err := n.available(); err != nil {
		return nil, err
	}
	base, _, err := n.storage.LogBase()
	if err != nil {
		return nil, n.fail(err)
	}
	if after < base {
		return nil, ErrRaftCompacted
	}
	if after > n.hard.Commit {
		return nil, ErrInvalidRaftUpdate
	}
	var entries []RaftEntry
	size := raftUpdateHeaderBytes
	for after < n.hard.Commit && len(entries) < maxRaftMessageEntries {
		entry, err := n.storage.Entry(after + 1)
		if err != nil {
			return nil, n.fail(err)
		}
		if size+raftEntryHeaderBytes+len(entry.Data) > maxJournalRecordBytes {
			break
		}
		entries = append(entries, entry)
		size += raftEntryHeaderBytes + len(entry.Data)
		after++
	}
	return entries, nil
}

func (n *RaftNode) vote(m RaftMessage) ([]RaftMessage, error) {
	last, err := n.storage.LastIndex()
	if err != nil {
		return nil, n.fail(err)
	}
	term, err := n.storage.Term(last)
	if err != nil {
		return nil, n.fail(err)
	}
	grant := (n.hard.Vote == 0 || n.hard.Vote == m.From) && (m.LogTerm > term || m.LogTerm == term && m.Index >= last)
	if grant {
		hard := n.hard
		hard.Vote = m.From
		if err := n.save(hard, nil); err != nil {
			return nil, err
		}
	}
	reply := n.message(RaftVoteResponse, m.From, 0, 0, 0)
	reply.Reject = !grant
	return []RaftMessage{reply}, nil
}

func (n *RaftNode) becomeLeader() ([]RaftMessage, error) {
	last, err := n.storage.LastIndex()
	if err != nil {
		return nil, n.fail(err)
	}
	if last == ^uint64(0) {
		return nil, n.fail(ErrRaftExhausted)
	}
	if err := n.save(n.hard, []RaftEntry{{Index: last + 1, Term: n.hard.Term}}); err != nil {
		return nil, n.fail(err)
	}
	n.role, n.leader = RaftLeader, n.config.NodeID
	n.votes = nil
	n.peers = make(map[uint64]*raftPeer)
	for _, id := range n.config.Voters {
		if id != n.config.NodeID {
			n.peers[id] = &raftPeer{next: last + 1}
		}
	}
	if err := n.advanceCommit(); err != nil {
		return nil, err
	}
	return n.broadcast()
}

func (n *RaftNode) appendRequest(m RaftMessage) ([]RaftMessage, error) {
	n.follow(m.From)
	reply := n.message(RaftAppendResponse, m.From, m.Request, m.Index, 0)
	last, err := n.storage.LastIndex()
	if err != nil {
		return nil, n.fail(err)
	}
	base, _, err := n.storage.LogBase()
	if err != nil {
		return nil, n.fail(err)
	}
	previousTerm, err := n.storage.Term(m.Index)
	if errors.Is(err, ErrRaftEntryMissing) || errors.Is(err, ErrRaftCompacted) || err == nil && previousTerm != m.LogTerm {
		reply.Reject = true
		reply.Hint = max(uint64(1), m.Index)
		if m.Index > last && last != ^uint64(0) {
			reply.Hint = last + 1
		}
		if m.Index < base && base != ^uint64(0) {
			reply.Hint = base + 1
		}
		return []RaftMessage{reply}, nil
	}
	if err != nil {
		return nil, n.fail(err)
	}
	matched := m.Index + uint64(len(m.Entries))
	hard := n.hard
	hard.Commit = max(hard.Commit, min(m.Commit, matched))
	if err := n.save(hard, m.Entries); err != nil {
		return nil, err
	}
	reply.Match = matched
	return []RaftMessage{reply}, nil
}

func (n *RaftNode) appendResponse(m RaftMessage) ([]RaftMessage, error) {
	if n.role != RaftLeader {
		return nil, nil
	}
	peer := n.peers[m.From]
	if peer == nil || peer.snapshot || peer.request == 0 || m.Request != peer.request || m.Index != peer.previous {
		return nil, nil
	}
	if m.Reject {
		if peer.previous <= peer.match {
			return nil, nil
		}
		last, err := n.storage.LastIndex()
		if err != nil {
			return nil, n.fail(err)
		}
		hint := m.Hint
		if hint == 0 {
			hint = max(uint64(1), peer.previous)
		}
		if hint > last {
			hint = last
		} // 失败响应不能跳过本地日志
		peer.next = max(peer.match+1, hint)
		peer.request = 0
		return n.sendAppend(m.From)
	}
	if m.Match != peer.sent {
		return nil, ErrInvalidRaftMessage
	}
	peer.match = max(peer.match, m.Match)
	peer.request = 0
	if peer.match == ^uint64(0) {
		peer.next = peer.match
	} else {
		peer.next = peer.match + 1
	}
	before := n.hard.Commit
	if err := n.advanceCommit(); err != nil {
		return nil, err
	}
	if n.hard.Commit != before {
		return n.broadcast()
	}
	last, err := n.storage.LastIndex()
	if err != nil {
		return nil, n.fail(err)
	}
	if peer.match < last {
		return n.sendAppend(m.From)
	}
	return nil, nil
}

func (n *RaftNode) advanceCommit() error {
	last, err := n.storage.LastIndex()
	if err != nil {
		return n.fail(err)
	}
	matches := make([]uint64, 0, len(n.config.Voters))
	matches = append(matches, last)
	for _, id := range n.config.Voters {
		if id != n.config.NodeID {
			matches = append(matches, n.peers[id].match)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i] < matches[j] })
	index := matches[(len(matches)-1)/2]
	if index <= n.hard.Commit {
		return nil
	}
	term, err := n.storage.Term(index)
	if err != nil {
		return n.fail(err)
	}
	if term != n.hard.Term {
		return nil
	}
	hard := n.hard
	hard.Commit = index
	return n.save(hard, nil)
}

func (n *RaftNode) broadcast() ([]RaftMessage, error) {
	var messages []RaftMessage
	for _, id := range n.config.Voters {
		if id == n.config.NodeID {
			continue
		}
		out, err := n.sendAppend(id)
		if err != nil {
			return nil, err
		}
		messages = append(messages, out...)
	}
	return messages, nil
}

func (n *RaftNode) sendAppend(id uint64) ([]RaftMessage, error) {
	peer := n.peers[id]
	base, _, err := n.storage.LogBase()
	if err != nil {
		return nil, n.fail(err)
	}
	if peer.next <= base {
		peer.snapshot = true
		return nil, nil
	}
	peer.snapshot = false
	previous := peer.next - 1
	if peer.match == ^uint64(0) {
		previous = peer.match
	}
	last, err := n.storage.LastIndex()
	if err != nil {
		return nil, n.fail(err)
	}
	request := peer.request
	if request != 0 {
		// 重发同一批次，避免高频提案或心跳持续作废仍在途的确认。
		previous, last = peer.previous, peer.sent
	} else {
		if n.sequence == ^uint64(0) {
			return nil, n.fail(ErrRaftExhausted)
		}
		n.sequence++
		request = n.sequence
	}
	term, err := n.storage.Term(previous)
	if err != nil {
		return nil, n.fail(err)
	}
	m := n.message(RaftAppendRequest, id, request, previous, term)
	m.Commit = n.hard.Commit
	size := raftUpdateHeaderBytes
	for index := previous + 1; previous != ^uint64(0) && index <= last && len(m.Entries) < maxRaftMessageEntries; index++ {
		entry, err := n.storage.Entry(index)
		if err != nil {
			return nil, n.fail(err)
		}
		if size+raftEntryHeaderBytes+len(entry.Data) > maxJournalRecordBytes {
			break
		}
		m.Entries = append(m.Entries, entry)
		size += raftEntryHeaderBytes + len(entry.Data)
		if index == ^uint64(0) {
			break
		}
	}
	peer.request, peer.previous, peer.sent = m.Request, previous, previous+uint64(len(m.Entries))
	return []RaftMessage{m}, nil
}

func (n *RaftNode) message(kind RaftMessageType, to, request, index, term uint64) RaftMessage {
	return RaftMessage{Cluster: n.cluster, Type: kind, From: n.config.NodeID, To: to, Term: n.hard.Term, Request: request, Index: index, LogTerm: term}
}

func (n *RaftNode) follow(leader uint64) {
	n.role, n.leader = RaftFollower, leader
	n.votes = nil
	clear(n.peers)
}

func (n *RaftNode) available() error {
	if n.failed != nil {
		return errors.Join(ErrRaftNodeFailed, n.failed)
	}
	_, err := n.storage.State()
	if err != nil {
		return n.fail(err)
	}
	return nil
}

func (n *RaftNode) fail(err error) error { n.failed = err; return err }

func (n *RaftNode) save(hard RaftHardState, entries []RaftEntry) error {
	err := n.storage.Save(RaftUpdate{HardState: hard, Entries: entries})
	if err != nil {
		if !errors.Is(err, ErrRaftCapacity) && !errors.Is(err, ErrJournalCapacity) && !errors.Is(err, ErrInvalidRaftUpdate) {
			n.fail(err)
		}
		return err
	}
	n.hard = hard
	return nil
}

func (n *RaftNode) validMessage(m RaftMessage) bool {
	if m.Cluster != n.cluster || m.To != n.config.NodeID || m.From == n.config.NodeID || m.Term == 0 {
		return false
	}
	member := sort.Search(len(n.config.Voters), func(i int) bool { return n.config.Voters[i] >= m.From })
	if member == len(n.config.Voters) || n.config.Voters[member] != m.From {
		return false
	}
	if m.Snapshot != nil && m.Type != RaftSnapshotRequest {
		return false
	}
	switch m.Type {
	case RaftVoteRequest:
		return m.Request == 0 && m.LogTerm <= m.Term && (m.Index == 0) == (m.LogTerm == 0) && m.Commit == 0 && len(m.Entries) == 0 && !m.Reject && m.Match == 0 && m.Hint == 0
	case RaftVoteResponse:
		return m.Request == 0 && m.Index == 0 && m.LogTerm == 0 && m.Commit == 0 && len(m.Entries) == 0 && m.Match == 0 && m.Hint == 0
	case RaftAppendResponse:
		return m.Request != 0 && m.LogTerm == 0 && m.Commit == 0 && len(m.Entries) == 0 && (m.Reject && m.Match == 0 || !m.Reject && m.Hint == 0)
	case RaftSnapshotRequest:
		return m.Request != 0 && m.Snapshot != nil && m.Index == m.Snapshot.Index && m.LogTerm == m.Snapshot.Term && m.LogTerm <= m.Term && m.Commit == 0 && len(m.Entries) == 0 && !m.Reject && m.Match == 0 && m.Hint == 0 && validateRaftSnapshot(*m.Snapshot) == nil
	case RaftSnapshotResponse:
		return m.Request != 0 && m.Index != 0 && m.LogTerm == 0 && m.Commit == 0 && len(m.Entries) == 0 && m.Hint == 0 && (m.Reject && m.Match == 0 || !m.Reject && m.Match == m.Index)
	case RaftAppendRequest:
		if m.Request == 0 || m.LogTerm > m.Term || (m.Index == 0) != (m.LogTerm == 0) || len(m.Entries) > maxRaftMessageEntries || m.Reject || m.Match != 0 || m.Hint != 0 {
			return false
		}
		index, term, size := m.Index, m.LogTerm, raftUpdateHeaderBytes
		for _, entry := range m.Entries {
			if index == ^uint64(0) || entry.Index != index+1 || entry.Term == 0 || entry.Term < term || entry.Term > m.Term || len(entry.Data) > maxRaftEntryBytes {
				return false
			}
			index, term = entry.Index, entry.Term
			size += raftEntryHeaderBytes + len(entry.Data)
			if size > maxJournalRecordBytes {
				return false
			}
		}
		return true
	}
	return false
}
