package clusterstore

import "errors"

var (
	ErrRaftReadNotReady = errors.New("raft leader has not committed an entry in its current term")
	ErrRaftReadPending  = errors.New("raft read confirmation is already pending")
)

// raftReadRound 只保留一轮多数派确认；后到的读取由运行时排入下一轮。
// Request 独立于复制批次，旧轮次、重复成员和其他任期的应答不能提供新确认。
type raftReadRound struct {
	request, term, index uint64
	acknowledged         map[uint64]bool
	ready                bool
}

func (n *RaftNode) beginRead() ([]RaftMessage, error) {
	if err := n.available(); err != nil {
		return nil, err
	}
	if n.role != RaftLeader {
		return nil, ErrRaftNotLeader
	}
	if n.read != nil {
		return nil, ErrRaftReadPending
	}
	term, err := n.storage.Term(n.hard.Commit)
	if err != nil {
		return nil, n.fail(err)
	}
	// 先提交本任期条目，才能知道所有已完成写入都包含在本地提交前缀中。
	if term != n.hard.Term {
		return nil, ErrRaftReadNotReady
	}
	if n.sequence == ^uint64(0) {
		return nil, ErrRaftExhausted
	}
	n.sequence++
	n.read = &raftReadRound{
		request: n.sequence, term: n.hard.Term, index: n.hard.Commit,
		acknowledged: map[uint64]bool{n.config.NodeID: true}, ready: len(n.config.Voters) == 1,
	}
	return n.retryRead(), nil
}

func (n *RaftNode) retryRead() []RaftMessage {
	if n.read == nil || n.read.ready || n.role != RaftLeader || n.read.term != n.hard.Term {
		return nil
	}
	var messages []RaftMessage
	for _, id := range n.config.Voters {
		if !n.read.acknowledged[id] {
			messages = append(messages, n.message(RaftReadRequest, id, n.read.request, n.read.index, 0))
		}
	}
	return messages
}

func (n *RaftNode) cancelRead(request uint64) {
	if n.read != nil && n.read.request == request {
		n.read = nil
	}
}

func (n *RaftNode) readRequest(m RaftMessage) []RaftMessage {
	n.follow(m.From)
	// 不推进日志、Match 或 Commit；确认当前任期中的联系即可，不要求副本已追平。
	return []RaftMessage{n.message(RaftReadResponse, m.From, m.Request, m.Index, 0)}
}

func (n *RaftNode) readResponse(m RaftMessage) {
	read := n.read
	if n.role != RaftLeader || read == nil || read.ready || m.Reject || m.Term != read.term || m.Request != read.request || m.Index != read.index {
		return
	}
	read.acknowledged[m.From] = true
	read.ready = len(read.acknowledged) > len(n.config.Voters)/2
}
