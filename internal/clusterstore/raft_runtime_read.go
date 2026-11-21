package clusterstore

import (
	"errors"
	"time"

	"github.com/kercylan98/vivid"
)

var (
	ErrRaftReadTimeout  = errors.New("raft read confirmation timed out")
	ErrRaftReadCapacity = errors.New("raft read wait capacity exceeded")
	ErrInvalidRaftRead  = errors.New("invalid raft read request")
)

// RaftRuntimeGet 在当前 Leader 上执行线性一致读取，不自动转发或重试。
// Timeout 为 0 时等待 5 秒，负值无效；外层 Ask 超时只结束调用方等待。
// 每轮重新确认多数派，不提供可在后续操作中复用的租约或写入授权。
type RaftRuntimeGet struct {
	Key     string
	Timeout time.Duration
}

// RaftRuntimeLocalGet 只返回本地已应用状态，可在 Follower 上诊断落后情况。
// 结果可能过期，不能代替 RaftRuntimeGet。
type RaftRuntimeLocalGet struct{ Key string }

type raftReadWait struct {
	sender   vivid.ActorRef
	key      string
	term     uint64
	round    uint64
	deadline time.Time
	control  bool
}

func (a *RaftActor) get(ctx vivid.ActorContext, message RaftRuntimeGet, now time.Time) {
	a.getRequest(ctx, message, now, false)
}

func (a *RaftActor) getRequest(ctx vivid.ActorContext, message RaftRuntimeGet, now time.Time, control bool) {
	err := a.available()
	if err == nil && (!validKey(message.Key) || message.Timeout < 0 || ctx.Sender() == nil) {
		err = ErrInvalidRaftRead
	}
	if err == nil && a.node.role != RaftLeader {
		err = ErrRaftNotLeader
	}
	if err == nil && len(a.reads) >= a.opts.MaxPendingReads {
		a.expireReads(ctx, now)
		if len(a.reads) >= a.opts.MaxPendingReads {
			err = ErrRaftReadCapacity
		}
	}
	if err != nil {
		a.replyRead(ctx, ctx.Sender(), RaftRuntimeRecord{Err: err}, control)
		return
	}
	timeout := message.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	a.reads = append(a.reads, raftReadWait{sender: ctx.Sender(), key: message.Key, term: a.node.hard.Term, deadline: now.Add(timeout), control: control})
}

func (a *RaftActor) expireReads(ctx vivid.ActorContext, now time.Time) {
	kept := a.reads[:0]
	for _, wait := range a.reads {
		if !now.Before(wait.deadline) {
			a.replyRead(ctx, wait.sender, RaftRuntimeRecord{Err: ErrRaftReadTimeout}, wait.control)
		} else {
			kept = append(kept, wait)
		}
	}
	clear(a.reads[len(kept):])
	a.reads = kept
}

// checkReads 在本轮消息的提交应用完成后执行，不能将确认位置当作已经应用。
func (a *RaftActor) checkReads(ctx vivid.ActorContext) {
	if len(a.reads) == 0 && a.readRequest == 0 {
		return
	}
	a.expireReads(ctx, time.Now())
	for {
		err := a.available()
		if err == nil && a.node.role != RaftLeader {
			err = ErrRaftNotLeader
		}
		if err == nil && len(a.reads) > 0 && a.reads[0].term != a.node.hard.Term {
			err = ErrRaftNotLeader
		}
		if err != nil {
			for _, wait := range a.reads {
				a.replyRead(ctx, wait.sender, RaftRuntimeRecord{Err: err}, wait.control)
			}
			a.reads = nil
			a.cancelRead()
			return
		}
		active := false
		for _, wait := range a.reads {
			active = active || a.readRequest != 0 && wait.round == a.readRequest
		}
		if !active {
			a.cancelRead()
		}
		if len(a.reads) == 0 {
			a.reads = nil
			return
		}
		if a.readRequest == 0 {
			out, err := a.node.beginRead()
			if errors.Is(err, ErrRaftReadNotReady) {
				return // 等待本任期空操作提交，原有超时预算继续消耗。
			}
			if err != nil {
				a.checkFailure()
				for _, wait := range a.reads {
					a.replyRead(ctx, wait.sender, RaftRuntimeRecord{Err: err}, wait.control)
				}
				a.reads = nil
				return
			}
			a.readRequest = a.node.read.request
			for i := range a.reads {
				a.reads[i].round = a.readRequest
			}
			a.messages(ctx, out)
			if a.available() != nil {
				continue
			}
		}
		read := a.node.read
		if read == nil || !read.ready || a.applied < read.index {
			return
		}
		kept := a.reads[:0]
		for _, wait := range a.reads {
			if wait.round == read.request {
				result := RaftRuntimeRecord{Err: ErrRaftReadTimeout}
				if time.Now().Before(wait.deadline) {
					result = RaftRuntimeRecord{Index: a.applied, Term: read.term}
					result.Record, result.Exists = a.store.Get(wait.key)
				}
				a.replyRead(ctx, wait.sender, result, wait.control)
			} else {
				kept = append(kept, wait)
			}
		}
		clear(a.reads[len(kept):])
		a.reads = kept
		a.cancelRead()
		// 下一轮只覆盖此时已经等待的请求；单节点确认可立即完成。
	}
}

func (a *RaftActor) cancelRead() {
	if a.node != nil {
		a.node.cancelRead(a.readRequest)
	}
	a.readRequest = 0
}
