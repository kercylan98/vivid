package clusterstore

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"time"
)

type snapshotTransferSession struct {
	id       SnapshotTransferID
	data     []byte
	offset   uint64
	deadline time.Time
}

// RaftSnapshotTransfer 串行驱动分块收发，每个 node 只使用一个传输器并定期调用 Expire。
// 只输出有界编码帧，不创建网络或定时任务；调用方在写出结束后释放帧，关闭时调用 Close。
// 完整收发缓冲共享额度，解析及安装仍可能申请额外内存，不提供总堆内存上限。
type RaftSnapshotTransfer struct {
	node      *RaftNode
	budget    *SnapshotTransferBudget
	outgoing  map[uint64]*snapshotTransferSession
	frames    map[uint64]*SnapshotFrame
	incoming  *snapshotTransferSession
	last      SnapshotTransferID
	completed bool
	closed    bool
}

func NewRaftSnapshotTransfer(node *RaftNode, budget *SnapshotTransferBudget) (*RaftSnapshotTransfer, error) {
	if node == nil || budget == nil {
		return nil, ErrSnapshotTransferClosed
	}
	if err := node.available(); err != nil {
		return nil, err
	}
	return &RaftSnapshotTransfer{node: node, budget: budget, outgoing: make(map[uint64]*snapshotTransferSession), frames: make(map[uint64]*SnapshotFrame)}, nil
}

// Send 开始或重发一个块；每个副本只有一个未确认块，仍在写出的帧不能再次生成。
// 重发不延长进度截止时间。容量不足可在其他会话或发送帧释放后重试。
func (t *RaftSnapshotTransfer) Send(id uint64, now time.Time) (*SnapshotFrame, error) {
	if err := t.Expire(now); err != nil {
		return nil, err
	}
	if session := t.outgoing[id]; session != nil {
		return t.sendFrame(session)
	}
	if frame := t.frames[id]; frame != nil && !frame.released.Load() {
		return nil, ErrSnapshotTransferBusy
	}
	if t.node.role != RaftLeader {
		return nil, ErrRaftNotLeader
	}
	peer := t.node.peers[id]
	if peer == nil || !peer.snapshot {
		return nil, ErrInvalidRaftUpdate
	}
	metadata, err := t.node.storage.SnapshotMetadata()
	if err != nil {
		return nil, t.node.fail(err)
	}
	if metadata.Size == 0 || metadata.Size > maxSnapshotBytes {
		return nil, ErrInvalidSnapshot
	}
	if err := t.budget.reserve(metadata.Size); err != nil {
		return nil, err
	}
	out, err := t.node.SendSnapshot(id)
	if err != nil {
		t.budget.release(metadata.Size)
		return nil, err
	}
	m := out[0]
	if m.Index != metadata.Index || m.LogTerm != metadata.Term || uint64(len(m.Snapshot.Data)) != metadata.Size {
		t.budget.release(metadata.Size)
		t.cancelRequest(id, m.Request)
		return nil, t.node.fail(ErrInvalidSnapshot)
	}
	identity := SnapshotTransferID{Cluster: m.Cluster, Digest: metadata.Digest, From: m.From, To: m.To, Term: m.Term, Request: m.Request, Index: m.Index, LogTerm: m.LogTerm, Size: metadata.Size}
	if _, err := rand.Read(identity.Session[:]); err != nil {
		t.budget.release(metadata.Size)
		t.cancelRequest(id, m.Request)
		return nil, err
	}
	session := &snapshotTransferSession{id: identity, data: m.Snapshot.Data, deadline: now.Add(t.budget.limits.Timeout)}
	t.outgoing[id] = session
	return t.sendFrame(session)
}

func (t *RaftSnapshotTransfer) sendFrame(session *snapshotTransferSession) (*SnapshotFrame, error) {
	end := min(session.offset+SnapshotChunkBytes, session.id.Size)
	return t.frame(session.id.To, SnapshotPacket{Type: SnapshotChunk, ID: session.id, Offset: session.offset, Data: session.data[session.offset:end]})
}

// Step 同步处理解码包；返回的 Raft 消息交回普通复制流程，返回的帧由调用方写出并释放。
// 块确认仅表示内存接收进度，完整快照校验并安装成功后才发 Done 并推进 Leader 的复制位置。
func (t *RaftSnapshotTransfer) Step(p SnapshotPacket, now time.Time) (*SnapshotFrame, []RaftMessage, error) {
	if err := t.Expire(now); err != nil {
		return nil, nil, err
	}
	if !p.valid() || p.ID.Cluster != t.node.cluster {
		return nil, nil, ErrInvalidSnapshotPacket
	}
	if p.Type == SnapshotChunk {
		if p.ID.To != t.node.config.NodeID || !t.member(p.ID.From) {
			return nil, nil, ErrInvalidSnapshotPacket
		}
		if err := t.node.observeTerm(p.ID.Term); err != nil {
			t.clear()
			return nil, nil, err
		}
		if err := t.Expire(now); err != nil {
			return nil, nil, err
		}
		frame, err := t.receive(p, now)
		return frame, nil, err
	}
	if p.ID.From != t.node.config.NodeID || !t.member(p.ID.To) {
		return nil, nil, ErrInvalidSnapshotPacket
	}
	if err := t.node.observeTerm(p.ReceiverTerm); err != nil {
		t.clear()
		return nil, nil, err
	}
	if err := t.Expire(now); err != nil {
		return nil, nil, err
	}
	return t.acknowledge(p, now)
}

func (t *RaftSnapshotTransfer) receive(p SnapshotPacket, now time.Time) (*SnapshotFrame, error) {
	if p.ID.Term != t.node.hard.Term {
		return t.reply(p.ID, 0, false, true)
	}
	if p.ID == t.last && t.completed {
		return t.reply(p.ID, p.ID.Size, true, false)
	}
	if t.incoming == nil || t.incoming.id != p.ID {
		if p.Offset != 0 || t.last.Term == p.ID.Term && (t.last.From != p.ID.From || p.ID.Request <= t.last.Request) {
			return t.reply(p.ID, 0, false, true)
		}
		t.dropIncoming()
		if err := t.budget.reserve(p.ID.Size); err != nil {
			return nil, err
		}
		t.node.follow(p.ID.From)
		t.last, t.completed = p.ID, false
		t.incoming = &snapshotTransferSession{id: p.ID, data: make([]byte, int(p.ID.Size)), deadline: now.Add(t.budget.limits.Timeout)}
	}
	session := t.incoming
	end := p.Offset + uint64(len(p.Data))
	if p.Offset < session.offset {
		if end > session.offset || !bytes.Equal(session.data[p.Offset:end], p.Data) {
			t.dropIncoming()
			return nil, ErrInvalidSnapshotPacket
		}
		return t.reply(p.ID, session.offset, false, false)
	}
	if p.Offset > session.offset {
		return t.reply(p.ID, session.offset, false, false)
	}
	copy(session.data[p.Offset:end], p.Data)
	session.offset, session.deadline = end, now.Add(t.budget.limits.Timeout)
	if end < p.ID.Size {
		return t.reply(p.ID, end, false, false)
	}
	if sha256.Sum256(session.data) != p.ID.Digest {
		t.dropIncoming()
		return nil, ErrInvalidSnapshotPacket
	}
	snapshot := RaftSnapshot{Index: p.ID.Index, Term: p.ID.LogTerm, Data: session.data}
	m := t.node.message(RaftSnapshotRequest, t.node.config.NodeID, p.ID.Request, p.ID.Index, p.ID.LogTerm)
	m.From, m.Term, m.Snapshot = p.ID.From, p.ID.Term, &snapshot
	_, err := t.node.Step(m)
	t.dropIncoming()
	if err != nil {
		return nil, err
	}
	t.completed = true
	return t.reply(p.ID, end, true, false)
}

func (t *RaftSnapshotTransfer) acknowledge(p SnapshotPacket, now time.Time) (*SnapshotFrame, []RaftMessage, error) {
	session := t.outgoing[p.ID.To]
	if session == nil || session.id != p.ID {
		return nil, nil, nil
	}
	if p.Reject {
		t.dropOutgoing(p.ID.To)
		return nil, nil, ErrSnapshotTransferStale
	}
	end := min(session.offset+SnapshotChunkBytes, p.ID.Size)
	if p.Offset <= session.offset {
		return nil, nil, nil
	}
	if p.Offset != end {
		return nil, nil, ErrInvalidSnapshotPacket
	}
	if p.Done {
		m := RaftMessage{Cluster: p.ID.Cluster, Type: RaftSnapshotResponse, From: p.ID.To, To: p.ID.From, Term: p.ID.Term, Request: p.ID.Request, Index: p.ID.Index, Match: p.ID.Index}
		out, err := t.node.Step(m)
		t.dropOutgoing(p.ID.To)
		return nil, out, err
	}
	session.offset, session.deadline = p.Offset, now.Add(t.budget.limits.Timeout)
	frame, err := t.sendFrame(session)
	return frame, nil, err
}

func (t *RaftSnapshotTransfer) reply(id SnapshotTransferID, offset uint64, done, reject bool) (*SnapshotFrame, error) {
	return t.frame(id.From, SnapshotPacket{Type: SnapshotChunkAck, ID: id, Offset: offset, Done: done, Reject: reject, ReceiverTerm: t.node.hard.Term})
}

func (t *RaftSnapshotTransfer) frame(id uint64, packet SnapshotPacket) (*SnapshotFrame, error) {
	if frame := t.frames[id]; frame != nil && !frame.released.Load() {
		return nil, ErrSnapshotTransferBusy
	}
	frame, err := t.budget.frame(packet)
	if err != nil {
		return nil, err
	}
	t.frames[id] = frame
	return frame, nil
}

func (t *RaftSnapshotTransfer) member(id uint64) bool {
	if id == t.node.config.NodeID {
		return false
	}
	for _, voter := range t.node.config.Voters {
		if voter == id {
			return true
		}
	}
	return false
}

// Expire 使用调用方的本地单调时间，清理超时、任期/角色变化或再次压缩失效的会话。
// 调用方应在普通 Raft 状态变更后调用，静默期间也需定期驱动；重发与重复块不延长截止时间。
func (t *RaftSnapshotTransfer) Expire(now time.Time) error {
	if t.closed {
		return ErrSnapshotTransferClosed
	}
	if now.IsZero() {
		return ErrInvalidSnapshotPacket
	}
	if err := t.node.available(); err != nil {
		t.clear()
		return err
	}
	for id, frame := range t.frames {
		if frame.released.Load() {
			delete(t.frames, id)
		}
	}
	for id, session := range t.outgoing {
		peer := t.node.peers[id]
		if t.node.role != RaftLeader || session.id.Term != t.node.hard.Term || peer == nil || !peer.snapshot || peer.request != session.id.Request || peer.previous != session.id.Index || !now.Before(session.deadline) {
			t.dropOutgoing(id)
		}
	}
	if t.incoming != nil {
		session := t.incoming
		if session.id.Term != t.node.hard.Term || t.node.role != RaftFollower || t.node.leader != session.id.From || !now.Before(session.deadline) {
			t.dropIncoming()
		}
	}
	if t.last.Term != t.node.hard.Term {
		t.last, t.completed = SnapshotTransferID{}, false
	}
	return nil
}

// Cancel 取消与该成员的当前会话，迟到旧块不得重新接纳；仍在写出的帧由其持有者释放。
func (t *RaftSnapshotTransfer) Cancel(id uint64) {
	if t.closed {
		return
	}
	t.dropOutgoing(id)
	if t.incoming != nil && t.incoming.id.From == id {
		t.dropIncoming()
	}
	if t.last.From == id {
		t.completed = false
	}
}

func (t *RaftSnapshotTransfer) cancelRequest(id, request uint64) {
	if peer := t.node.peers[id]; peer != nil && peer.snapshot && peer.request == request {
		peer.request = 0
	}
}

func (t *RaftSnapshotTransfer) dropOutgoing(id uint64) {
	if session := t.outgoing[id]; session != nil {
		t.cancelRequest(id, session.id.Request)
		t.budget.release(session.id.Size)
		session.data = nil
		delete(t.outgoing, id)
	}
}

func (t *RaftSnapshotTransfer) dropIncoming() {
	if t.incoming != nil {
		t.budget.release(t.incoming.id.Size)
		t.incoming.data = nil
		t.incoming = nil
	}
}

func (t *RaftSnapshotTransfer) clear() {
	for id := range t.outgoing {
		t.dropOutgoing(id)
	}
	t.dropIncoming()
}

func (t *RaftSnapshotTransfer) Close() {
	if t.closed {
		return
	}
	t.clear()
	t.frames = nil
	t.node, t.last, t.completed, t.closed = nil, SnapshotTransferID{}, false, true
}
