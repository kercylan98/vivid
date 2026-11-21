package actor

import (
	"context"
	"errors"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/pkg/log"
)

type virtualOwnershipBackend interface {
	claim(context.Context, virtualKey) (clusterstore.VirtualOwnership, error)
	validate(context.Context, clusterstore.VirtualOwnership) error
	release(context.Context, clusterstore.VirtualOwnership) error
}

// virtualOwnershipClient 暂用明确的本地 Raft 控制入口，不猜测 Leader、转发或重试写入。
// 所有者与控制入口可以属于不同系统；此处只支持同进程调用，副本间仍由 Raft 负责传输。
type virtualOwnershipClient struct {
	system *System
	ref    vivid.ActorRef
	owner  clusterstore.VirtualOwner
}

func newVirtualOwnershipClient(system *System, ref vivid.ActorRef, owner clusterstore.VirtualOwner) (*virtualOwnershipClient, error) {
	if system == nil || ref == nil || ref.IsVirtual() {
		return nil, vivid.ErrorIllegalArgument
	}
	system.statusLock.Lock()
	available := system.status == start && system.Context != nil
	var address string
	if available {
		address = system.Context.ref.GetAddress()
	}
	system.statusLock.Unlock()
	if !available || ref.GetAddress() != address {
		return nil, vivid.ErrorIllegalArgument
	}
	if _, err := clusterstore.ClaimVirtualOwnership("validate", "owner", owner, 0); err != nil {
		return nil, err
	}
	return &virtualOwnershipClient{system: system, ref: ref.Clone(), owner: owner}, nil
}

func (c *virtualOwnershipClient) ask(ctx context.Context, message vivid.Message, write bool) (vivid.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	request := c.system.Ask(c.ref, message, 0)
	stop := context.AfterFunc(ctx, func() { request.Close(ctx.Err()) })
	defer stop()
	result, err := request.Result()
	if err != nil && write {
		err = errors.Join(clusterstore.ErrRaftOutcomeUnknown, err)
	}
	return result, err
}

func ownershipTimeout(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 5 * time.Second
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return time.Nanosecond
	}
	return remaining
}

func (c *virtualOwnershipClient) claim(ctx context.Context, key virtualKey) (clusterstore.VirtualOwnership, error) {
	change, err := clusterstore.ClaimVirtualOwnership(key.kind, key.name, c.owner, 0)
	if err != nil {
		return clusterstore.VirtualOwnership{}, err
	}
	result, err := c.ask(ctx, change.Request(ownershipTimeout(ctx)), true)
	if err != nil {
		return clusterstore.VirtualOwnership{}, err
	}
	execution, ok := result.(clusterstore.RaftRuntimeExecution)
	if !ok {
		return clusterstore.VirtualOwnership{}, errors.Join(clusterstore.ErrRaftOutcomeUnknown, clusterstore.ErrInvalidVirtualOwnership)
	}
	return change.Result(execution)
}

func (c *virtualOwnershipClient) validate(ctx context.Context, ownership clusterstore.VirtualOwnership) error {
	key, err := clusterstore.VirtualOwnershipKey(ownership.Kind, ownership.Name)
	if err != nil {
		return err
	}
	result, err := c.ask(ctx, clusterstore.RaftRuntimeGet{Key: key, Timeout: ownershipTimeout(ctx)}, false)
	if err != nil {
		return err
	}
	record, ok := result.(clusterstore.RaftRuntimeRecord)
	if !ok {
		return clusterstore.ErrInvalidVirtualOwnership
	}
	current, exists, err := clusterstore.ReadVirtualOwnership(ownership.Kind, ownership.Name, record)
	if err == nil && (!exists || current != ownership) {
		err = clusterstore.ErrVirtualOwnershipConflict
	}
	return err
}

func (c *virtualOwnershipClient) release(ctx context.Context, ownership clusterstore.VirtualOwnership) error {
	change, err := ownership.Release()
	if err != nil {
		return err
	}
	result, err := c.ask(ctx, change.Request(ownershipTimeout(ctx)), true)
	if err != nil {
		return err
	}
	execution, ok := result.(clusterstore.RaftRuntimeExecution)
	if !ok {
		return errors.Join(clusterstore.ErrRaftOutcomeUnknown, clusterstore.ErrInvalidVirtualOwnership)
	}
	_, err = change.Result(execution)
	return err
}

type virtualOwnershipGate struct {
	backend  virtualOwnershipBackend
	timeout  time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	deadline time.Time // 停止时所有释放任务共用此预算，不逐任务延长
	done     chan struct{}
	finished bool
}

type virtualOwnershipState struct {
	value    clusterstore.VirtualOwnership
	deadline time.Time
	retiring bool
	after    func()
}

// configureOwnership 只在系统启动前安装内部准入策略，不开放尚未完整的公共集群配置。
func (v *virtualCoordinator) configureOwnership(backend virtualOwnershipBackend, timeout time.Duration) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.system.statusLock.Lock()
	defer v.system.statusLock.Unlock()
	if backend == nil || timeout < 0 || v.system.status != ready || v.gate != nil {
		return vivid.ErrorIllegalArgument
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.gate = &virtualOwnershipGate{backend: backend, timeout: timeout, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	return nil
}

func (v *virtualCoordinator) claimOwnership(entry *virtualActivation) error {
	ctx, cancel := context.WithTimeout(v.gate.ctx, v.gate.timeout)
	defer cancel()
	ownership, err := v.gate.backend.claim(ctx, entry.key)
	if err != nil {
		return err
	}
	v.mu.Lock()
	entry.ownership = &virtualOwnershipState{value: ownership}
	closed := v.closed
	v.mu.Unlock()
	if closed {
		return vivid.ErrorActorSystemStopped
	}
	return v.gate.backend.validate(ctx, ownership)
}

func (v *virtualCoordinator) validateOwnership(entry *virtualActivation) error {
	ctx, cancel := context.WithTimeout(v.gate.ctx, v.gate.timeout)
	defer cancel()
	return v.gate.backend.validate(ctx, entry.ownership.value)
}

// retireLocked 仅登记；释放与激活共用有界并发，绝不在协调器锁内等待 Raft。
func (v *virtualCoordinator) retireLocked(entry *virtualActivation) {
	state := entry.ownership
	if state.retiring {
		return
	}
	state.retiring = true
	state.deadline = time.Now().Add(v.gate.timeout)
	if v.closed && v.gate.deadline.Before(state.deadline) {
		state.deadline = v.gate.deadline
	}
	entry.mailbox = nil
	v.retiring = append(v.retiring, entry)
}

func (v *virtualCoordinator) releaseOwnership(entry *virtualActivation) {
	state := entry.ownership
	ctx, cancel := context.WithDeadline(context.Background(), state.deadline)
	err := v.gate.backend.release(ctx, state.value)
	cancel()
	v.mu.Lock()
	entry.ownership = nil
	after := state.after
	var queued []virtualDelivery
	if err == nil && len(entry.queue) > 0 && !v.closed {
		v.waiting = append(v.waiting, entry)
	} else {
		delete(v.entries, entry.key)
		queued, entry.queue = entry.queue, nil
		v.pending -= len(queued)
	}
	v.mu.Unlock()
	if after != nil {
		after()
	}
	if err != nil {
		v.system.Logger().Error("virtual actor ownership release failed", log.String("kind", entry.key.kind), log.String("name", entry.key.name), log.Any("error", err))
	}
	for _, delivery := range queued {
		v.fail(delivery, err)
	}
	v.mu.Lock()
	v.running--
	v.startLocked()
	v.finishOwnershipLocked()
	v.mu.Unlock()
}

func (v *virtualCoordinator) finishOwnershipLocked() {
	if v.gate != nil && v.closed && !v.gate.finished && v.running == 0 && len(v.entries) == 0 {
		v.gate.finished = true
		close(v.gate.done)
	}
}
