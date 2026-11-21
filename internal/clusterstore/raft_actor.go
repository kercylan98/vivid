package clusterstore

import (
	"errors"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/remoting"
)

var ErrRaftRuntimeClosed = errors.New("raft runtime is closed")

// RaftActorOptions 的 Routes 绑定固定 NodeID 与通信引用，每个投票节点使用独立 ActorSystem。
// Create 只用于首次显式创建；同一实例重启时恢复已有 WAL，不自动重建缺失文件。
type RaftActorOptions struct {
	Path                 string
	Config               RaftConfig
	Create               bool
	Routes               map[uint64]vivid.ActorRef
	HeartbeatInterval    time.Duration
	ElectionMin          time.Duration
	ElectionMax          time.Duration
	MaxPendingBytes      uint64 // 本驱动持有的协议字节，不含 Codec、网络队列和恢复状态机分配
	MaxPendingExecutions int    // 本地等待提交及执行回执的请求数
	MaxPendingReads      int    // 等待多数派确认及应用的读取请求数，0 使用 1024
	SnapshotBudget       *SnapshotTransferBudget
}

type RaftRuntimeStatusRequest struct{}
type RaftRuntimeCompact struct{}

// RaftRuntimeLocalLog 查询本地已提交日志，仅用于恢复诊断。
type RaftRuntimeLocalLog struct{ After uint64 }
type RaftRuntimeRoute struct {
	ID  uint64
	Ref vivid.ActorRef
}

// RaftRuntimeResult 返回内部压缩及路由维护结果；协调写入使用 RaftRuntimeExecution。
type RaftRuntimeResult struct {
	Err error
}

type RaftRuntimeStatus struct {
	RaftStatus
	Applied           uint64
	PendingBytes      uint64
	PendingExecutions int
	PendingReads      int
	Err               error
}

// RaftRuntimeRecord 的 Index 是读取时的应用位置；返回值与内部状态互不别名。
// RaftRuntimeGet 成功时 Term 标识本轮多数派确认的任期；本地查询的 Term 为 0。
// 确认仅适用于本次读取，不保证返回时仍有多数派，也不能授权下一次写入。
type RaftRuntimeRecord struct {
	Record
	Exists bool
	Index  uint64
	Term   uint64
	Err    error
}

// RaftRuntimeLog 是本地已提交日志副本，落后于压缩基准时同时返回需要先恢复的快照。
// 用于内部恢复诊断，不提供线性一致读。
type RaftRuntimeLog struct {
	Snapshot *RaftSnapshot
	Entries  []RaftEntry
	Err      error
}

type raftActorLifetime struct{ closed atomic.Bool }
type raftTransportSystem interface {
	vivid.ActorLiaison
	ResolveMailbox(vivid.ActorRef) vivid.Mailbox
}
type raftActorTick struct{ lifetime *raftActorLifetime }
type raftWrite struct {
	peer      uint64
	size      uint64
	keepalive bool
	read      bool
	snapshot  SnapshotTransferID
	offset    uint64
}
type raftWriteDone struct {
	lifetime *raftActorLifetime
	write    *raftWrite
}

// RaftActor 用所属邮箱串行驱动固定成员共识，复用系统 Scheduler 和 Remoting。
// 串行应用已提交协调命令，不将 Leader 状态当作多数派租约；错误通过状态查询暴露。
type RaftActor struct {
	opts              RaftActorOptions
	storage           *RaftStorage
	node              *RaftNode
	store             *Store
	applied           uint64
	transfer          *RaftSnapshotTransfer
	lifetime          *raftActorLifetime
	timer             vivid.ScheduledTask
	pending           map[uint64]*raftWrite
	pendingBytes      uint64
	readPending       map[uint64]*raftWrite // 与复制分别持有每成员一个写出，共用字节额度
	reads             []raftReadWait
	readRequest       uint64
	executions        map[uint64]raftExecutionWait
	executionTerm     uint64
	electionDeadline  time.Time
	heartbeatDeadline time.Time
	status            RaftStatus
	err               error
}

func NewRaftActor(options RaftActorOptions) (*RaftActor, error) {
	config, err := normalizeRaftConfig(options.Config)
	if err != nil {
		return nil, err
	}
	if options.HeartbeatInterval == 0 {
		options.HeartbeatInterval = 100 * time.Millisecond
	}
	if options.ElectionMin == 0 {
		options.ElectionMin = time.Second
	}
	if options.ElectionMax == 0 {
		options.ElectionMax = 2 * time.Second
	}
	if options.MaxPendingBytes == 0 {
		options.MaxPendingBytes = 16 << 20
	}
	if options.MaxPendingExecutions == 0 {
		options.MaxPendingExecutions = 1024
	}
	if options.MaxPendingReads == 0 {
		options.MaxPendingReads = 1024
	}
	if options.Path == "" || options.HeartbeatInterval <= 0 || options.ElectionMin <= 0 || options.HeartbeatInterval > options.ElectionMin/3 || options.ElectionMax <= options.ElectionMin || options.MaxPendingBytes < maxRaftPacketBytes || options.MaxPendingExecutions < 1 || options.MaxPendingReads < 1 || len(options.Routes) != len(config.Voters) {
		return nil, ErrInvalidRaftConfig
	}
	routes := make(map[uint64]vivid.ActorRef, len(options.Routes))
	addresses := make(map[string]bool, len(options.Routes))
	for _, id := range config.Voters {
		ref := options.Routes[id]
		if ref == nil || ref.IsVirtual() || ref.GetAddress() == "" || len(ref.String()) > maxRaftRouteBytes || addresses[ref.GetAddress()] {
			return nil, ErrInvalidRaftConfig
		}
		addresses[ref.GetAddress()] = true
		routes[id] = ref.Clone()
	}
	if options.SnapshotBudget == nil {
		options.SnapshotBudget, err = NewSnapshotTransferBudget(SnapshotTransferLimits{})
		if err != nil {
			return nil, err
		}
	}
	options.Config, options.Routes = config, routes
	return &RaftActor{opts: options, status: RaftStatus{ID: config.NodeID}}, nil
}

func (a *RaftActor) OnReceive(ctx vivid.ActorContext) {
	defer func() {
		a.apply(ctx)
		a.checkExecutions(ctx)
		a.checkReads(ctx)
	}()
	switch message := ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.start(ctx)
	case *vivid.OnKill:
		a.close()
	case *vivid.OnKilled:
		if message.Ref.Equals(ctx.Ref()) {
			a.close()
		}
	case RaftRuntimeStatusRequest:
		if a.node != nil {
			status, err := a.node.Status()
			if err != nil {
				a.fail(err)
			} else {
				a.status = status
			}
		}
		ctx.Reply(RaftRuntimeStatus{RaftStatus: a.status, Applied: a.applied, PendingBytes: a.pendingBytes, PendingExecutions: len(a.executions), PendingReads: len(a.reads), Err: a.err})
	case RaftRuntimeExecute:
		a.execute(ctx, message, time.Now())
	case *RaftControlRequest:
		if message != nil {
			a.control(ctx, message, time.Now())
		}
	case RaftRuntimeCompact:
		err := a.available()
		if err == nil {
			err = a.node.Compact(a.store.Snapshot())
			if err == nil {
				err = a.transfer.Expire(time.Now())
			}
			a.checkFailure()
		}
		ctx.Reply(RaftRuntimeResult{Err: err})
	case RaftRuntimeLocalLog:
		a.read(ctx, message.After)
	case RaftRuntimeGet:
		a.get(ctx, message, time.Now())
	case RaftRuntimeLocalGet:
		result := RaftRuntimeRecord{Index: a.applied, Err: a.available()}
		if result.Err == nil {
			if !validKey(message.Key) {
				result.Err = ErrInvalidCommand
			} else {
				result.Record, result.Exists = a.store.Get(message.Key)
			}
		}
		ctx.Reply(result)
	case RaftRuntimeRoute:
		ctx.Reply(RaftRuntimeResult{Err: a.route(message)})
	case raftActorTick:
		if message.lifetime == a.lifetime && a.available() == nil {
			now := time.Now()
			a.expireExecutions(ctx, now)
			a.tick(ctx, now)
			a.schedule(ctx)
		}
	case raftWriteDone:
		a.written(ctx, message)
	case *RaftWire:
		if a.available() == nil {
			a.receive(ctx, message.Data, time.Now())
		}
	}
}

func (a *RaftActor) start(ctx vivid.ActorContext) {
	if a.lifetime != nil && !a.lifetime.closed.Load() {
		return
	}
	a.lifetime, a.err = &raftActorLifetime{}, nil
	a.pending, a.pendingBytes = make(map[uint64]*raftWrite), 0
	a.readPending = make(map[uint64]*raftWrite)
	if _, ok := ctx.System().(raftTransportSystem); !ok {
		a.fail(ErrInvalidRaftConfig)
		return
	}
	if !a.opts.Routes[a.opts.Config.NodeID].Equals(ctx.Ref()) {
		a.fail(ErrInvalidRaftConfig)
		return
	}
	var err error
	if a.opts.Create {
		a.storage, err = CreateRaftStorage(a.opts.Path, a.opts.Config)
	} else {
		a.storage, err = OpenRaftStorage(a.opts.Path, a.opts.Config)
	}
	if err != nil {
		a.fail(err)
		return
	}
	a.opts.Create = false
	a.node, err = NewRaftNode(a.storage)
	if err == nil {
		a.transfer, err = NewRaftSnapshotTransfer(a.node, a.opts.SnapshotBudget)
	}
	if err != nil {
		a.fail(err)
		return
	}
	a.store, a.applied = &Store{}, 0
	a.apply(ctx)
	if a.available() != nil {
		return
	}
	a.resetElection(time.Now())
	a.schedule(ctx)
}

func (a *RaftActor) resetElection(now time.Time) {
	a.electionDeadline = now.Add(a.opts.ElectionMin + time.Duration(rand.Int64N(int64(a.opts.ElectionMax-a.opts.ElectionMin))))
}

func (a *RaftActor) schedule(ctx vivid.ActorContext) {
	if a.available() != nil {
		return
	}
	timer, err := ctx.Scheduler().Once(ctx.Ref(), a.opts.HeartbeatInterval, raftActorTick{lifetime: a.lifetime}, vivid.WithSchedulerReference("raft-tick"))
	if err != nil {
		a.fail(err)
		return
	}
	a.timer = timer
}

func (a *RaftActor) tick(ctx vivid.ActorContext, now time.Time) {
	if err := a.transfer.Expire(now); err != nil {
		a.fail(err)
		return
	}
	if a.node.role != RaftLeader {
		if !now.Before(a.electionDeadline) {
			out, err := a.node.Campaign()
			if err != nil {
				a.fail(err)
				return
			}
			a.resetElection(now)
			a.messages(ctx, out)
		}
		return
	}
	if now.Before(a.heartbeatDeadline) {
		return
	}
	a.heartbeatDeadline = now.Add(a.opts.HeartbeatInterval)
	a.messages(ctx, a.node.retryRead())
	out, err := a.node.Heartbeat()
	if err != nil {
		a.fail(err)
		return
	}
	a.messages(ctx, out)
	if a.available() != nil {
		return
	}
	for _, id := range a.opts.Config.Voters {
		peer := a.node.peers[id]
		if peer == nil || !peer.snapshot || a.pending[id] != nil {
			continue
		}
		// 快照写出或进度超时不能阻断 Leader 活动；独立请求不会覆盖快照批次身份。
		if a.node.sequence == ^uint64(0) {
			a.fail(ErrRaftExhausted)
			return
		}
		a.node.sequence++
		keepalive := a.node.message(RaftAppendRequest, id, a.node.sequence, 0, 0)
		a.message(ctx, keepalive, true)
	}
}

func (a *RaftActor) receive(ctx vivid.ActorContext, data []byte, now time.Time) {
	beforeTerm, beforeRole := a.node.hard.Term, a.node.role
	activity := false
	if len(data) >= 8 && string(data[:8]) == snapshotPacketMagic {
		packet, err := DecodeSnapshotPacket(data)
		if err != nil {
			return
		}
		from := packet.ID.From
		if packet.Type == SnapshotChunkAck {
			from = packet.ID.To
		}
		if !a.sender(ctx, from) {
			return
		}
		frame, out, err := a.transfer.Step(packet, now)
		if frame != nil {
			a.frame(ctx, from, frame)
		}
		activity = packet.Type == SnapshotChunk && packet.ID.Term == a.node.hard.Term && a.node.leader == from
		if err == nil {
			a.messages(ctx, out)
		}
	} else {
		message, err := DecodeRaftPacket(data)
		if err != nil || !a.sender(ctx, message.From) || !a.node.validMessage(message) {
			return
		}
		out, err := a.node.Step(message)
		if err == nil {
			activity = message.Term == a.node.hard.Term && (message.Type == RaftAppendRequest || message.Type == RaftReadRequest || message.Type == RaftVoteRequest && len(out) == 1 && !out[0].Reject)
			a.messages(ctx, out)
		}
	}
	a.checkFailure()
	if a.available() != nil {
		return
	}
	if activity || a.node.hard.Term != beforeTerm || a.node.role != beforeRole {
		a.resetElection(now)
	}
	if a.node.role == RaftLeader && beforeRole != RaftLeader {
		a.heartbeatDeadline = now.Add(a.opts.HeartbeatInterval)
	}
	if err := a.transfer.Expire(now); err != nil {
		a.fail(err)
	}
}

func (a *RaftActor) sender(ctx vivid.ActorContext, id uint64) bool {
	ref := a.opts.Routes[id]
	return id != a.opts.Config.NodeID && ref != nil && ctx.Sender() != nil && ref.Equals(ctx.Sender())
}

func (a *RaftActor) messages(ctx vivid.ActorContext, messages []RaftMessage) {
	for _, message := range messages {
		a.message(ctx, message, false)
	}
}

func (a *RaftActor) message(ctx vivid.ActorContext, message RaftMessage, keepalive bool) {
	read := message.Type == RaftReadRequest || message.Type == RaftReadResponse
	pending := a.pending
	if read {
		pending = a.readPending
	}
	if a.available() != nil || pending[message.To] != nil {
		return
	}
	size, err := raftPacketSize(message)
	if err != nil {
		a.fail(err)
		return
	}
	if uint64(size) > a.opts.MaxPendingBytes-a.pendingBytes {
		return
	}
	data, err := EncodeRaftPacket(message)
	if err != nil {
		a.fail(err)
		return
	}
	a.send(ctx, &raftWrite{peer: message.To, size: uint64(size), keepalive: keepalive, read: read}, data, nil)
}

func (a *RaftActor) frame(ctx vivid.ActorContext, peer uint64, frame *SnapshotFrame) {
	if a.available() != nil || a.pending[peer] != nil || uint64(len(frame.Bytes)) > a.opts.MaxPendingBytes-a.pendingBytes {
		frame.Release()
		return
	}
	packet, err := DecodeSnapshotPacket(frame.Bytes)
	if err != nil {
		frame.Release()
		a.fail(err)
		return
	}
	a.send(ctx, &raftWrite{peer: peer, size: uint64(len(frame.Bytes)), snapshot: packet.ID, offset: packet.Offset}, frame.Bytes, frame)
}

func (a *RaftActor) send(ctx vivid.ActorContext, write *raftWrite, data []byte, frame *SnapshotFrame) {
	if write.read {
		a.readPending[write.peer] = write
	} else {
		a.pending[write.peer] = write
	}
	a.pendingBytes += write.size
	system, self, lifetime := ctx.System().(raftTransportSystem), ctx.Ref(), a.lifetime
	envelop := mailbox.NewEnvelop(false, self, a.opts.Routes[write.peer], &RaftWire{Data: data})
	system.ResolveMailbox(a.opts.Routes[write.peer]).Enqueue(remoting.NewOwnedEnvelop(envelop, func(bool) {
		frame.Release()
		if !lifetime.closed.Load() {
			system.Tell(self, raftWriteDone{lifetime: lifetime, write: write})
		}
	}))
}

func (a *RaftActor) written(ctx vivid.ActorContext, message raftWriteDone) {
	pending := a.pending
	if message.write.read {
		pending = a.readPending
	}
	if message.lifetime != a.lifetime || a.available() != nil || pending[message.write.peer] != message.write {
		return
	}
	delete(pending, message.write.peer)
	a.pendingBytes -= message.write.size
	if message.write.read {
		return
	}
	peer := a.node.peers[message.write.peer]
	if peer == nil || !peer.snapshot {
		return
	}
	session := a.transfer.outgoing[message.write.peer]
	if message.write.keepalive || session != nil && session.id == message.write.snapshot && session.offset > message.write.offset {
		frame, err := a.transfer.Send(message.write.peer, time.Now())
		if err == nil {
			a.frame(ctx, message.write.peer, frame)
		}
		a.checkFailure()
	}
}

func (a *RaftActor) read(ctx vivid.ActorContext, after uint64) {
	result := RaftRuntimeLog{Err: a.available()}
	if result.Err == nil {
		base, _, err := a.storage.LogBase()
		result.Err = err
		if err == nil && after < base {
			snapshot, err := a.node.Snapshot()
			result.Snapshot, result.Err, after = &snapshot, err, snapshot.Index
		}
		if result.Err == nil {
			result.Entries, result.Err = a.node.Committed(after)
		}
		a.checkFailure()
	}
	ctx.Reply(result)
}

func (a *RaftActor) route(message RaftRuntimeRoute) error {
	if err := a.available(); err != nil {
		return err
	}
	if message.ID == a.opts.Config.NodeID || a.opts.Routes[message.ID] == nil || message.Ref == nil || message.Ref.IsVirtual() || len(message.Ref.String()) > maxRaftRouteBytes || message.Ref.GetAddress() == "" {
		return ErrInvalidRaftConfig
	}
	for id, ref := range a.opts.Routes {
		if id != message.ID && ref.GetAddress() == message.Ref.GetAddress() {
			return ErrInvalidRaftConfig
		}
	}
	if a.opts.Routes[message.ID].Equals(message.Ref) {
		return nil
	}
	a.opts.Routes[message.ID] = message.Ref.Clone()
	a.transfer.Cancel(message.ID)
	return nil
}

func (a *RaftActor) available() error {
	if a.err != nil {
		return a.err
	}
	if a.node == nil || a.lifetime == nil || a.lifetime.closed.Load() {
		return ErrRaftRuntimeClosed
	}
	return nil
}

func (a *RaftActor) checkFailure() {
	if a.node != nil && a.node.failed != nil {
		a.fail(a.node.failed)
	}
}

func (a *RaftActor) apply(ctx vivid.ActorContext) {
	if a.available() != nil || a.applied == a.node.hard.Commit {
		return
	}
	var applied func(RaftEntry, Result)
	if len(a.executions) != 0 {
		applied = func(entry RaftEntry, result Result) {
			a.appliedExecution(ctx, entry, result, time.Now())
		}
	}
	store, err := replayRaftStore(a.node, a.store, applied)
	a.store, a.applied = store, store.Index()
	if err != nil {
		a.fail(err)
	}
}

func (a *RaftActor) fail(err error) {
	a.close()
	a.err = errors.Join(err, a.err)
}

func (a *RaftActor) close() {
	if a.lifetime == nil || a.lifetime.closed.Swap(true) {
		return
	}
	if a.timer != nil {
		_ = a.timer.Cancel()
		a.timer = nil
	}
	if a.node != nil {
		if status, err := a.node.Status(); err == nil {
			a.status = status
		}
	}
	if a.transfer != nil {
		a.transfer.Close()
	}
	a.err = ErrRaftRuntimeClosed
	if a.storage != nil {
		a.err = errors.Join(a.err, a.storage.Close())
	}
	a.node, a.storage, a.transfer, a.pending, a.pendingBytes = nil, nil, nil, nil, 0
	a.readPending = nil
	a.store = nil
}
