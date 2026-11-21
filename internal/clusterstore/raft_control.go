package clusterstore

import (
	"crypto/sha256"
	"errors"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
)

var ErrInvalidRaftControl = errors.New("invalid raft control message")

type RaftControlOperation uint8

const (
	RaftControlLocate RaftControlOperation = iota + 1
	RaftControlExecute
	RaftControlGet
)

// RaftControlGroup 绑定组名与固定投票成员，与共识消息使用相同边界，不提供认证。
func RaftControlGroup(config RaftConfig) ([sha256.Size]byte, error) {
	config, err := normalizeRaftConfig(config)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	config.NodeID = 0
	return sha256.Sum256(encodeRaftConfig(config)), nil
}

// RaftControlRequest 只开放定位、CAS 执行和一致性读取，不开放诊断、改路由或压缩。
// Timeout 是远端接纳后的等待预算；调用方总预算仍须包含传输和联络等待。
type RaftControlRequest struct {
	Group     [sha256.Size]byte
	Node      uint64
	Operation RaftControlOperation
	Timeout   time.Duration
	Command   Command
	Key       string
}

func (m *RaftControlRequest) valid() bool {
	if m.Group == [sha256.Size]byte{} || m.Node == 0 || m.Timeout < 0 {
		return false
	}
	switch m.Operation {
	case RaftControlLocate:
		return m.Key == "" && m.Command.Operation == Noop && m.Command.Key == "" && len(m.Command.Value) == 0 && m.Command.ExpectedRevision == 0
	case RaftControlExecute:
		return m.Key == "" && m.Command.Operation != Noop && validateCommand(m.Command) == nil
	case RaftControlGet:
		return validKey(m.Key) && m.Command.Operation == Noop && m.Command.Key == "" && len(m.Command.Value) == 0 && m.Command.ExpectedRevision == 0
	}
	return false
}

func (*RaftControlRequest) Encode(w *serialization.Writer, message any) error {
	m := message.(*RaftControlRequest)
	if !m.valid() {
		return ErrInvalidRaftControl
	}
	w.Write(uint8(1), m.Group, m.Node, uint8(m.Operation), int64(m.Timeout), uint8(m.Command.Operation), m.Command.ExpectedRevision, m.Command.Key, m.Command.Value, m.Key)
	return w.Err()
}

func (*RaftControlRequest) Decode(r *serialization.Reader, message any) error {
	var m RaftControlRequest
	var version uint8
	if err := r.Read(&version, &m.Group, &m.Node, &m.Operation, &m.Timeout, &m.Command.Operation, &m.Command.ExpectedRevision, &m.Command.Key, &m.Command.Value, &m.Key); err != nil {
		return err
	}
	if version != 1 || !m.valid() {
		return ErrInvalidRaftControl
	}
	*message.(*RaftControlRequest) = m
	return nil
}

type raftControlError uint8

const (
	controlOK raftControlError = iota
	controlNotLeader
	controlUnknown
	controlInvalid
	controlClosed
	controlUnavailable
	controlExecutionCapacity
	controlReadCapacity
	controlReadTimeout
	controlStateCapacity
	controlLogCapacity
)

func controlError(err error) raftControlError {
	switch {
	case err == nil:
		return controlOK
	case errors.Is(err, ErrRaftOutcomeUnknown):
		return controlUnknown
	case errors.Is(err, ErrRaftNotLeader):
		return controlNotLeader
	case errors.Is(err, ErrRaftRuntimeClosed):
		return controlClosed
	case errors.Is(err, ErrRaftExecutionCapacity):
		return controlExecutionCapacity
	case errors.Is(err, ErrRaftReadCapacity):
		return controlReadCapacity
	case errors.Is(err, ErrRaftReadTimeout):
		return controlReadTimeout
	case errors.Is(err, ErrCapacity):
		return controlStateCapacity
	case errors.Is(err, ErrRaftCapacity), errors.Is(err, ErrJournalCapacity), errors.Is(err, ErrRaftExhausted):
		return controlLogCapacity
	case errors.Is(err, ErrInvalidCommand), errors.Is(err, ErrInvalidRaftExecution), errors.Is(err, ErrInvalidRaftRead), errors.Is(err, ErrInvalidRaftControl):
		return controlInvalid
	default:
		return controlUnavailable
	}
}

func (e raftControlError) err() error {
	switch e {
	case controlOK:
		return nil
	case controlNotLeader:
		return ErrRaftNotLeader
	case controlUnknown:
		return ErrRaftOutcomeUnknown
	case controlInvalid:
		return ErrInvalidRaftControl
	case controlClosed:
		return ErrRaftRuntimeClosed
	case controlExecutionCapacity:
		return ErrRaftExecutionCapacity
	case controlReadCapacity:
		return ErrRaftReadCapacity
	case controlReadTimeout:
		return ErrRaftReadTimeout
	case controlStateCapacity:
		return ErrCapacity
	case controlLogCapacity:
		return ErrRaftCapacity
	default:
		return ErrRaftNodeFailed
	}
}

// RaftControlResponse 使用有限错误码，不在线上传输 Go error 或底层 I/O 诊断。
// Leader 与 Role 仅为联系线索；成功执行及读取仍由原有多数派协议决定。
type RaftControlResponse struct {
	Group           [sha256.Size]byte
	Node, Leader    uint64
	Operation       RaftControlOperation
	Role            RaftRole
	Index, Term     uint64
	Revision        uint64
	Applied, Exists bool
	Value           []byte
	code, result    raftControlError
}

func (m *RaftControlResponse) Err() error { return m.code.err() }

func (m *RaftControlResponse) Execution() RaftRuntimeExecution {
	if m.Operation != RaftControlExecute {
		return RaftRuntimeExecution{Err: ErrInvalidRaftControl}
	}
	return RaftRuntimeExecution{Index: m.Index, Term: m.Term, Result: Result{Applied: m.Applied, Revision: m.Revision, Err: m.result.err()}, Err: m.Err()}
}

func (m *RaftControlResponse) Record() RaftRuntimeRecord {
	if m.Operation != RaftControlGet {
		return RaftRuntimeRecord{Err: ErrInvalidRaftControl}
	}
	return RaftRuntimeRecord{Record: Record{Value: m.Value, Revision: m.Revision}, Exists: m.Exists, Index: m.Index, Term: m.Term, Err: m.Err()}
}

func (m *RaftControlResponse) valid() bool {
	if m.Group == [sha256.Size]byte{} || m.Node == 0 || m.Operation < RaftControlLocate || m.Operation > RaftControlGet || m.Role > RaftLeader || m.code > controlLogCapacity || len(m.Value) > maxValueBytes {
		return false
	}
	if m.code != controlOK {
		return !m.Applied && !m.Exists && m.Revision == 0 && len(m.Value) == 0 && m.result == controlOK && (m.Operation != RaftControlExecute || m.code != controlNotLeader || m.Index == 0 && m.Term == 0)
	}
	switch m.Operation {
	case RaftControlLocate:
		return m.Index == 0 && m.Revision == 0 && !m.Applied && !m.Exists && len(m.Value) == 0 && m.result == controlOK
	case RaftControlExecute:
		return m.Index > 0 && m.Term > 0 && !m.Exists && len(m.Value) == 0 && m.Revision <= m.Index && (m.result == controlOK || m.result == controlStateCapacity) && (!m.Applied || m.result == controlOK && (m.Revision == 0 || m.Revision == m.Index))
	case RaftControlGet:
		return m.Index > 0 && m.Term > 0 && !m.Applied && m.result == controlOK && (m.Exists && m.Revision > 0 && m.Revision <= m.Index || !m.Exists && m.Revision == 0 && len(m.Value) == 0)
	}
	return false
}

func (*RaftControlResponse) Encode(w *serialization.Writer, message any) error {
	m := message.(*RaftControlResponse)
	if !m.valid() {
		return ErrInvalidRaftControl
	}
	w.Write(uint8(1), m.Group, m.Node, m.Leader, uint8(m.Operation), uint8(m.Role), m.Index, m.Term, m.Revision, m.Applied, m.Exists, m.Value, uint8(m.code), uint8(m.result))
	return w.Err()
}

func (*RaftControlResponse) Decode(r *serialization.Reader, message any) error {
	var m RaftControlResponse
	var version uint8
	if err := r.Read(&version, &m.Group, &m.Node, &m.Leader, &m.Operation, &m.Role, &m.Index, &m.Term, &m.Revision, &m.Applied, &m.Exists, &m.Value, &m.code, &m.result); err != nil {
		return err
	}
	if version != 1 || !m.valid() {
		return ErrInvalidRaftControl
	}
	*message.(*RaftControlResponse) = m
	return nil
}

func (a *RaftActor) controlResponse(operation RaftControlOperation) *RaftControlResponse {
	m := &RaftControlResponse{Node: a.opts.Config.NodeID, Operation: operation}
	if a.node != nil {
		m.Group = a.node.cluster
		m.Role, m.Leader, m.Term = a.node.role, a.node.leader, a.node.hard.Term
	} else {
		m.Group, _ = RaftControlGroup(a.opts.Config)
	}
	return m
}

func (a *RaftActor) control(ctx vivid.ActorContext, m *RaftControlRequest, now time.Time) {
	response := a.controlResponse(m.Operation)
	if !m.valid() || m.Group != response.Group || m.Node != response.Node {
		response.code = controlInvalid
		// 直接进程内的非法操作也只返回可编码的拒绝。
		if m.Operation < RaftControlLocate || m.Operation > RaftControlGet {
			response.Operation = RaftControlLocate
		}
		ctx.Reply(response)
		return
	}
	switch m.Operation {
	case RaftControlLocate:
		response.code = controlError(a.available())
		ctx.Reply(response)
	case RaftControlExecute:
		a.executeRequest(ctx, RaftRuntimeExecute{Command: m.Command, Timeout: m.Timeout}, now, true)
	case RaftControlGet:
		a.getRequest(ctx, RaftRuntimeGet{Key: m.Key, Timeout: m.Timeout}, now, true)
	}
}

func (a *RaftActor) replyExecution(ctx vivid.ActorContext, sender vivid.ActorRef, result RaftRuntimeExecution, control bool) {
	if !control {
		ctx.Tell(sender, result)
		return
	}
	m := a.controlResponse(RaftControlExecute)
	m.Index, m.Term, m.Revision, m.Applied = result.Index, result.Term, result.Result.Revision, result.Result.Applied
	m.code, m.result = controlError(result.Err), controlError(result.Result.Err)
	ctx.Tell(sender, m)
}

func (a *RaftActor) replyRead(ctx vivid.ActorContext, sender vivid.ActorRef, result RaftRuntimeRecord, control bool) {
	if !control {
		ctx.Tell(sender, result)
		return
	}
	m := a.controlResponse(RaftControlGet)
	m.Index, m.Term, m.Revision, m.Exists, m.Value = result.Index, result.Term, result.Revision, result.Exists, result.Value
	m.code = controlError(result.Err)
	ctx.Tell(sender, m)
}
