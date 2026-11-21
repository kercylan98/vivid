package clusterstore

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/require"
)

type executionTestRef struct{ vivid.ActorRef }

type executionTestContext struct {
	vivid.ActorContext
	sender  vivid.ActorRef
	replies []RaftRuntimeExecution
}

func (c *executionTestContext) Sender() vivid.ActorRef      { return c.sender }
func (c *executionTestContext) Reply(message vivid.Message) { c.Tell(c.sender, message) }
func (c *executionTestContext) Tell(_ vivid.ActorRef, message vivid.Message) {
	c.replies = append(c.replies, message.(RaftRuntimeExecution))
}

func newExecutionDriver(t *testing.T) (*RaftActor, *executionTestContext) {
	t.Helper()
	network := newRaftNetwork(t, 1)
	network.campaign(t, 1)
	driver := &RaftActor{
		node: network.nodes[1], store: &Store{}, lifetime: &raftActorLifetime{},
		opts: RaftActorOptions{MaxPendingExecutions: 3},
	}
	ctx := &executionTestContext{sender: &executionTestRef{}}
	driver.apply(ctx)
	return driver, ctx
}

func TestRaftExecution_ActualResults(t *testing.T) {
	driver, ctx := newExecutionDriver(t)
	for _, command := range []Command{
		{Operation: Put, Key: "owner", Value: []byte("a")},
		{Operation: Put, Key: "owner", Value: []byte("b"), ExpectedRevision: 2},
		{Operation: Put, Key: "owner", Value: []byte("conflict"), ExpectedRevision: 2},
	} {
		driver.execute(ctx, RaftRuntimeExecute{Command: command}, time.Now())
	}
	require.Empty(t, ctx.replies, "本地保存和提交完成后仍须等待状态机实际执行")
	require.Len(t, driver.executions, 3)
	driver.apply(ctx)
	require.Len(t, ctx.replies, 3)
	for i, result := range []Result{{Applied: true, Revision: 2}, {Applied: true, Revision: 3}, {Revision: 3}} {
		require.NoError(t, ctx.replies[i].Err)
		require.Equal(t, result, ctx.replies[i].Result, "回执不能从最终记录倒推各条 CAS 的执行结果")
		require.EqualValues(t, i+2, ctx.replies[i].Index)
	}
	require.Empty(t, driver.executions)
	driver.apply(ctx)
	driver.expireExecutions(ctx, time.Now().Add(time.Hour))
	require.Len(t, ctx.replies, 3, "重复应用或迟到超时不能再次回复")
}

func TestRaftExecution_Admission(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request RaftRuntimeExecute
		prepare func(*RaftActor, *executionTestContext)
		want    error
	}{
		{name: "invalid timeout", request: RaftRuntimeExecute{Command: Command{Operation: Put, Key: "owner"}, Timeout: -1}, want: ErrInvalidRaftExecution},
		{name: "invalid command", want: ErrInvalidCommand},
		{name: "no sender", prepare: func(_ *RaftActor, c *executionTestContext) { c.sender = nil }, want: ErrInvalidRaftExecution},
		{name: "follower", prepare: func(a *RaftActor, _ *executionTestContext) { a.node.follow(0) }, want: ErrRaftNotLeader},
		{name: "capacity", prepare: func(a *RaftActor, c *executionTestContext) {
			a.opts.MaxPendingExecutions = 1
			a.executions = map[uint64]raftExecutionWait{2: {sender: c.sender, term: 1, deadline: time.Now().Add(time.Minute)}}
		}, want: ErrRaftExecutionCapacity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver, ctx := newExecutionDriver(t)
			if tc.prepare != nil {
				tc.prepare(driver, ctx)
			}
			driver.execute(ctx, tc.request, time.Now())
			require.Len(t, ctx.replies, 1)
			require.ErrorIs(t, ctx.replies[0].Err, tc.want)
			require.NotErrorIs(t, ctx.replies[0].Err, ErrRaftOutcomeUnknown)
			last, err := driver.node.storage.LastIndex()
			require.NoError(t, err)
			require.EqualValues(t, 1, last, "确定拒绝时不能追加日志")
		})
	}
}

func TestRaftExecution_TimeoutDoesNotCancel(t *testing.T) {
	for _, expireBeforeApply := range []bool{false, true} {
		driver, ctx := newExecutionDriver(t)
		driver.execute(ctx, RaftRuntimeExecute{Command: Command{Operation: Put, Key: "owner", Value: []byte("a")}, Timeout: time.Millisecond}, time.Now().Add(-time.Second))
		if expireBeforeApply {
			driver.expireExecutions(ctx, time.Now())
		}
		driver.apply(ctx)
		require.Len(t, ctx.replies, 1)
		require.ErrorIs(t, ctx.replies[0].Err, ErrRaftOutcomeUnknown)
		require.ErrorIs(t, ctx.replies[0].Err, ErrRaftExecutionTimeout)
		require.Equal(t, Result{}, ctx.replies[0].Result)
		record, exists := driver.store.Get("owner")
		require.True(t, exists, "等待超时不撤销已经提交的命令")
		require.Equal(t, []byte("a"), record.Value)
		require.Empty(t, driver.executions)
	}
}

func TestRaftExecution_ExpiredCapacityReused(t *testing.T) {
	driver, ctx := newExecutionDriver(t)
	driver.opts.MaxPendingExecutions = 1
	driver.execute(ctx, RaftRuntimeExecute{Command: Command{Operation: Put, Key: "old"}, Timeout: time.Millisecond}, time.Now().Add(-time.Second))
	now := time.Now()
	driver.execute(ctx, RaftRuntimeExecute{Command: Command{Operation: Put, Key: "new"}}, now)
	require.Len(t, ctx.replies, 1)
	require.ErrorIs(t, ctx.replies[0].Err, ErrRaftExecutionTimeout)
	require.Len(t, driver.executions, 1)
	require.Equal(t, now.Add(5*time.Second), driver.executions[3].deadline)
	driver.apply(ctx)
	require.Len(t, ctx.replies, 2)
	require.NoError(t, ctx.replies[1].Err)
	require.True(t, ctx.replies[1].Result.Applied)
	require.Empty(t, driver.executions)
}

func TestRaftExecution_BusinessCapacityResult(t *testing.T) {
	driver, ctx := newExecutionDriver(t)
	for i := range 31 {
		proposeCommand(t, driver.node, Command{Operation: Put, Key: fmt.Sprint(i), Value: make([]byte, maxValueBytes)})
	}
	driver.apply(ctx)
	driver.execute(ctx, RaftRuntimeExecute{Command: Command{Operation: Put, Key: "overflow", Value: make([]byte, maxValueBytes)}}, time.Now())
	driver.apply(ctx)
	require.Len(t, ctx.replies, 1)
	require.NoError(t, ctx.replies[0].Err, "容量拒绝也是确定的已提交执行结果")
	require.ErrorIs(t, ctx.replies[0].Result.Err, ErrCapacity)
	require.False(t, ctx.replies[0].Result.Applied)
	require.Equal(t, ctx.replies[0].Index, driver.applied)
	require.NoError(t, driver.available())
}

func TestRaftExecution_OverwrittenIndex(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	driver := &RaftActor{node: network.nodes[1], store: &Store{}, lifetime: &raftActorLifetime{}}
	ctx := &executionTestContext{sender: &executionTestRef{}}
	driver.apply(ctx)
	index := proposeCommand(t, driver.node, Command{Operation: Put, Key: "old", Value: []byte("minority")})
	driver.executions = map[uint64]raftExecutionWait{index: {sender: ctx.sender, term: driver.node.hard.Term, deadline: time.Now().Add(time.Minute)}}
	driver.executionTerm = driver.node.hard.Term
	network.blocked[1] = true
	network.campaign(t, 2)
	network.drain(t)
	data, err := EncodeCommand(Command{Operation: Put, Key: "new", Value: []byte("majority")})
	require.NoError(t, err)
	_, out, err := network.nodes[2].Propose(data)
	require.NoError(t, err)
	network.queue = append(network.queue, out...)
	network.drain(t)
	clear(network.blocked)
	out, err = network.nodes[2].Heartbeat()
	require.NoError(t, err)
	network.queue = append(network.queue, out...)
	network.drain(t)
	driver.apply(ctx)
	driver.checkExecutions(ctx)
	require.Len(t, ctx.replies, 1)
	require.ErrorIs(t, ctx.replies[0].Err, ErrRaftOutcomeUnknown)
	require.Equal(t, Result{}, ctx.replies[0].Result, "同一位置的新任期空操作不能确认旧命令成功")
	_, exists := driver.store.Get("old")
	require.False(t, exists)
	record, exists := driver.store.Get("new")
	require.True(t, exists)
	require.Equal(t, []byte("majority"), record.Value)
}

func TestRaftExecution_LeadershipAndClose(t *testing.T) {
	for _, closeDriver := range []bool{false, true} {
		driver, ctx := newExecutionDriver(t)
		driver.executions = map[uint64]raftExecutionWait{2: {sender: ctx.sender, term: 1, deadline: time.Now().Add(time.Minute)}}
		driver.executionTerm = 1
		if closeDriver {
			driver.close()
		} else {
			driver.node.follow(0)
		}
		driver.checkExecutions(ctx)
		driver.checkExecutions(ctx)
		require.Len(t, ctx.replies, 1)
		require.ErrorIs(t, ctx.replies[0].Err, ErrRaftOutcomeUnknown)
		require.Empty(t, driver.executions)
	}
	// 任期变化的同一轮消息已提交并应用原命令时，仍可返回它的确定结果。
	driver, ctx := newExecutionDriver(t)
	driver.execute(ctx, RaftRuntimeExecute{Command: Command{Operation: Put, Key: "owner"}}, time.Now())
	require.NoError(t, driver.node.observeTerm(2))
	driver.apply(ctx)
	driver.checkExecutions(ctx)
	require.Len(t, ctx.replies, 1)
	require.NoError(t, ctx.replies[0].Err)
	require.True(t, ctx.replies[0].Result.Applied)
}

type executionFailureStorage struct {
	*memoryRaftStorage
	commitErr error
}

func (s *executionFailureStorage) Save(update RaftUpdate) error {
	if s.commitErr != nil && len(update.Entries) == 0 && update.HardState.Commit > s.state.hard.Commit {
		return s.commitErr
	}
	return s.memoryRaftStorage.Save(update)
}

func TestRaftExecution_SaveFailure(t *testing.T) {
	for _, commitErr := range []error{ErrJournalCapacity, io.ErrUnexpectedEOF} {
		driver, ctx := newExecutionDriver(t)
		storage := &executionFailureStorage{memoryRaftStorage: driver.node.storage.(*memoryRaftStorage), commitErr: commitErr}
		driver.node.storage = storage
		driver.execute(ctx, RaftRuntimeExecute{Command: Command{Operation: Put, Key: "owner"}}, time.Now())
		require.Len(t, ctx.replies, 1)
		require.ErrorIs(t, ctx.replies[0].Err, ErrRaftOutcomeUnknown)
		require.ErrorIs(t, ctx.replies[0].Err, commitErr)
		require.EqualValues(t, 2, ctx.replies[0].Index, "保存成功后提交失败也必须保留位置，不能当作确定拒绝")
		require.EqualValues(t, 2, storage.state.lastIndex())
		require.EqualValues(t, 1, storage.state.hard.Commit)
	}
}
