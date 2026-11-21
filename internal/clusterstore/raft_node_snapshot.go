package clusterstore

import "errors"

// Snapshot 返回最近持久化的快照副本；应用位置落后于基准时应先恢复它，再消费 Committed。
func (n *RaftNode) Snapshot() (RaftSnapshot, error) {
	if err := n.available(); err != nil {
		return RaftSnapshot{}, err
	}
	snapshot, err := n.storage.Snapshot()
	if err != nil {
		return RaftSnapshot{}, n.fail(err)
	}
	return snapshot, nil
}

// Compact 安装本节点已应用的快照，失效已经落在压缩范围内的在途复制批次。
// 调用方负责提供对应位置的状态机内容，不能将尚未应用的 Commit 当作快照位置。
func (n *RaftNode) Compact(data []byte) error {
	if err := n.available(); err != nil {
		return err
	}
	if err := n.storage.Compact(data); err != nil {
		return n.snapshotError(err)
	}
	base, _, err := n.storage.LogBase()
	if err != nil {
		return n.fail(err)
	}
	for _, peer := range n.peers {
		if peer.request != 0 && peer.previous < base {
			peer.request = 0
		}
		peer.snapshot = peer.next <= base
	}
	return nil
}

// SendSnapshot 为需要快照的副本生成或重发一个完整快照，不在普通心跳中读取快照文件。
// 每份消息归调用方所有；负载受 maxSnapshotBytes 限制，调用方须限制在途数量，尚不提供分块传输。
func (n *RaftNode) SendSnapshot(id uint64) ([]RaftMessage, error) {
	if err := n.available(); err != nil {
		return nil, err
	}
	if n.role != RaftLeader {
		return nil, ErrRaftNotLeader
	}
	peer := n.peers[id]
	if peer == nil || !peer.snapshot {
		return nil, ErrInvalidRaftUpdate
	}
	snapshot, err := n.Snapshot()
	if err != nil {
		return nil, err
	}
	if snapshot.Index == 0 {
		return nil, ErrInvalidRaftUpdate
	}
	if peer.request == 0 {
		if n.sequence == ^uint64(0) {
			return nil, n.fail(ErrRaftExhausted)
		}
		n.sequence++
		peer.request = n.sequence
	}
	peer.previous, peer.sent = snapshot.Index, snapshot.Index
	m := n.message(RaftSnapshotRequest, id, peer.request, snapshot.Index, snapshot.Term)
	m.Snapshot = &snapshot
	return []RaftMessage{m}, nil
}

func (n *RaftNode) snapshotRequest(m RaftMessage) ([]RaftMessage, error) {
	n.follow(m.From)
	reply := n.message(RaftSnapshotResponse, m.From, m.Request, m.Index, 0)
	if m.Index > n.hard.Commit {
		if err := n.storage.InstallSnapshot(*m.Snapshot); err != nil {
			return nil, n.snapshotError(err)
		}
		hard, err := n.storage.State()
		if err != nil {
			return nil, n.fail(err)
		}
		n.hard = hard
	} else {
		// 已提交的位置不能被旧快照替换；仍保留本地日志时核对其末尾任期。
		base, _, err := n.storage.LogBase()
		if err != nil {
			return nil, n.fail(err)
		}
		if m.Index >= base {
			term, err := n.storage.Term(m.Index)
			if err != nil {
				return nil, n.fail(err)
			}
			if term != m.LogTerm {
				return nil, ErrInvalidRaftUpdate
			}
		}
	}
	reply.Match = m.Index
	return []RaftMessage{reply}, nil
}

func (n *RaftNode) snapshotResponse(m RaftMessage) ([]RaftMessage, error) {
	if n.role != RaftLeader {
		return nil, nil
	}
	peer := n.peers[m.From]
	if peer == nil || !peer.snapshot || peer.request == 0 || m.Request != peer.request || m.Index != peer.previous {
		return nil, nil
	}
	peer.request = 0
	if m.Reject {
		return nil, nil
	}
	peer.snapshot = false
	peer.match = max(peer.match, m.Match)
	peer.next = peer.match
	if peer.next != ^uint64(0) {
		peer.next++
	}
	return n.sendAppend(m.From)
}

func (n *RaftNode) snapshotError(err error) error {
	if !errors.Is(err, ErrInvalidSnapshot) && !errors.Is(err, ErrInvalidRaftUpdate) && !errors.Is(err, ErrRaftCapacity) && !errors.Is(err, ErrJournalCapacity) {
		n.fail(err)
	}
	return err
}
