package clusterstore

import (
	"errors"
	"time"

	"github.com/kercylan98/vivid"
)

var (
	ErrRaftOutcomeUnknown    = errors.New("raft command outcome is unknown")
	ErrRaftExecutionTimeout  = errors.New("raft execution wait timed out")
	ErrRaftExecutionCapacity = errors.New("raft execution wait capacity exceeded")
	ErrInvalidRaftExecution  = errors.New("invalid raft execution request")
)

// RaftRuntimeExecute 等待当前 Leader 将命令提交并执行，不自动转发或重试。
// Timeout 为 0 时等待 5 秒，负值无效；超时检查复用心跳，不能打断存储或消息处理。
// 消息及 Command.Value 在接纳前不可修改，外层 Ask 的超时不会撤销已接纳命令。
type RaftRuntimeExecute struct {
	Command Command
	Timeout time.Duration
}

// RaftRuntimeExecution 的 Err 为 nil 时，Result 是该条已提交命令的实际执行结果。
// CAS 冲突表现为 Result.Applied=false；容量不足由 Result.Err 返回。
// ErrRaftOutcomeUnknown 表示命令可能已生效，不可当作失败后盲目重试；Index/Term 只标识本次日志。
type RaftRuntimeExecution struct {
	Index, Term uint64
	Result      Result
	Err         error
}

type raftExecutionWait struct {
	sender   vivid.ActorRef
	term     uint64
	deadline time.Time
}

func (a *RaftActor) execute(ctx vivid.ActorContext, message RaftRuntimeExecute, now time.Time) {
	result := RaftRuntimeExecution{Err: a.available()}
	if result.Err == nil && (message.Timeout < 0 || ctx.Sender() == nil) {
		result.Err = ErrInvalidRaftExecution
	}
	if result.Err == nil && a.node.role != RaftLeader {
		result.Err = ErrRaftNotLeader
	}
	if result.Err == nil && len(a.executions) >= a.opts.MaxPendingExecutions {
		a.expireExecutions(ctx, now)
		if len(a.executions) >= a.opts.MaxPendingExecutions {
			result.Err = ErrRaftExecutionCapacity
		}
	}
	if result.Err != nil {
		ctx.Reply(result)
		return
	}
	data, err := EncodeCommand(message.Command)
	if err == nil && len(data) == 0 {
		err = ErrInvalidCommand
	}
	if err != nil {
		ctx.Reply(RaftRuntimeExecution{Err: err})
		return
	}
	result.Term = a.node.hard.Term
	var out []RaftMessage
	result.Index, out, result.Err = a.node.Propose(data)
	if result.Err != nil {
		// I/O 失败可能已落盘；广播失败也可能发生在本地保存或提交之后。
		rejected := result.Index == 0 && (errors.Is(result.Err, ErrRaftCapacity) || errors.Is(result.Err, ErrJournalCapacity) || errors.Is(result.Err, ErrInvalidRaftUpdate) || errors.Is(result.Err, ErrRaftExhausted))
		if !rejected {
			result.Err = errors.Join(ErrRaftOutcomeUnknown, result.Err)
		}
		ctx.Reply(result)
		a.checkFailure()
		return
	}
	if message.Timeout == 0 {
		message.Timeout = 5 * time.Second
	}
	if a.executions == nil {
		a.executions = make(map[uint64]raftExecutionWait)
	}
	a.executions[result.Index] = raftExecutionWait{sender: ctx.Sender(), term: result.Term, deadline: now.Add(message.Timeout)}
	a.executionTerm = result.Term
	a.messages(ctx, out)
	a.checkFailure()
}

func (a *RaftActor) appliedExecution(ctx vivid.ActorContext, entry RaftEntry, result Result, now time.Time) {
	wait, exists := a.executions[entry.Index]
	if !exists {
		return
	}
	response := RaftRuntimeExecution{Index: entry.Index, Term: wait.term, Result: result}
	if wait.term != entry.Term {
		response.Err = ErrRaftOutcomeUnknown
	} else if !now.Before(wait.deadline) {
		response.Err = errors.Join(ErrRaftOutcomeUnknown, ErrRaftExecutionTimeout)
	}
	if response.Err != nil {
		response.Result = Result{}
	}
	delete(a.executions, entry.Index)
	ctx.Tell(wait.sender, response)
}

func (a *RaftActor) expireExecutions(ctx vivid.ActorContext, now time.Time) {
	for index, wait := range a.executions {
		if !now.Before(wait.deadline) {
			a.endExecution(ctx, index, wait, ErrRaftExecutionTimeout)
		}
	}
}

func (a *RaftActor) checkExecutions(ctx vivid.ActorContext) {
	if len(a.executions) == 0 {
		return
	}
	err := a.available()
	if err == nil && (a.node.role != RaftLeader || a.node.hard.Term != a.executionTerm) {
		err = ErrRaftNotLeader
	}
	if err == nil {
		return
	}
	for index, wait := range a.executions {
		a.endExecution(ctx, index, wait, err)
	}
}

func (a *RaftActor) endExecution(ctx vivid.ActorContext, index uint64, wait raftExecutionWait, err error) {
	delete(a.executions, index)
	ctx.Tell(wait.sender, RaftRuntimeExecution{Index: index, Term: wait.term, Err: errors.Join(ErrRaftOutcomeUnknown, err)})
}
