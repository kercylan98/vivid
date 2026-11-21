package clusterstore

import (
	"io"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/require"
)

type readTestMailbox struct {
	vivid.Mailbox
	envelopes []vivid.Envelop
}

func (m *readTestMailbox) Enqueue(envelope vivid.Envelop) {
	m.envelopes = append(m.envelopes, envelope)
}

type readTestSystem struct {
	vivid.ActorSystem
	mailbox readTestMailbox
}

func (s *readTestSystem) ResolveMailbox(vivid.ActorRef) vivid.Mailbox { return &s.mailbox }
func (s *readTestSystem) Tell(vivid.ActorRef, vivid.Message)          {}

type readTestContext struct {
	vivid.ActorContext
	sender  vivid.ActorRef
	system  readTestSystem
	replies []RaftRuntimeRecord
}

func (c *readTestContext) System() vivid.ActorSystem   { return &c.system }
func (c *readTestContext) Ref() vivid.ActorRef         { return c.sender }
func (c *readTestContext) Sender() vivid.ActorRef      { return c.sender }
func (c *readTestContext) Reply(message vivid.Message) { c.Tell(c.sender, message) }
func (c *readTestContext) Tell(_ vivid.ActorRef, message vivid.Message) {
	c.replies = append(c.replies, message.(RaftRuntimeRecord))
}

func newReadDriver(t *testing.T, count int) (*RaftActor, *readTestContext, *raftNetwork) {
	t.Helper()
	network := newRaftNetwork(t, count)
	network.campaign(t, 1)
	network.drain(t)
	driver := &RaftActor{
		node: network.nodes[1], store: &Store{}, lifetime: &raftActorLifetime{},
		opts:        RaftActorOptions{MaxPendingReads: 3, MaxPendingBytes: 16 << 20, Routes: make(map[uint64]vivid.ActorRef)},
		readPending: make(map[uint64]*raftWrite), pending: make(map[uint64]*raftWrite),
	}
	for _, id := range network.nodes[1].config.Voters {
		driver.opts.Routes[id] = &executionTestRef{}
	}
	ctx := &readTestContext{sender: &executionTestRef{}}
	driver.apply(ctx)
	return driver, ctx, network
}

func TestRaftRuntimeGet_Admission(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request RaftRuntimeGet
		prepare func(*RaftActor, *readTestContext)
		want    error
	}{
		{name: "key", want: ErrInvalidRaftRead},
		{name: "timeout", request: RaftRuntimeGet{Key: "owner", Timeout: -1}, want: ErrInvalidRaftRead},
		{name: "sender", request: RaftRuntimeGet{Key: "owner"}, prepare: func(_ *RaftActor, c *readTestContext) { c.sender = nil }, want: ErrInvalidRaftRead},
		{name: "follower", request: RaftRuntimeGet{Key: "owner"}, prepare: func(a *RaftActor, _ *readTestContext) { a.node.follow(0) }, want: ErrRaftNotLeader},
		{name: "capacity", request: RaftRuntimeGet{Key: "owner"}, prepare: func(a *RaftActor, c *readTestContext) {
			a.opts.MaxPendingReads = 1
			a.reads = []raftReadWait{{sender: c.sender, key: "owner", term: a.node.hard.Term, deadline: time.Now().Add(time.Minute)}}
		}, want: ErrRaftReadCapacity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver, ctx, _ := newReadDriver(t, 1)
			if tc.prepare != nil {
				tc.prepare(driver, ctx)
			}
			driver.get(ctx, tc.request, time.Now())
			require.Len(t, ctx.replies, 1)
			require.ErrorIs(t, ctx.replies[0].Err, tc.want)
			require.Zero(t, ctx.replies[0].Term)
			require.Empty(t, ctx.replies[0].Value)
		})
	}
}

func TestRaftRuntimeGet_BatchingAndAppliedBoundary(t *testing.T) {
	driver, ctx, network := newReadDriver(t, 3)
	request := RaftRuntimeGet{Key: "owner"}
	driver.get(ctx, request, time.Now())
	driver.checkReads(ctx)
	require.Len(t, ctx.system.mailbox.envelopes, 2)
	firstRound := driver.readRequest
	require.NotZero(t, firstRound)
	driver.get(ctx, request, time.Now()) // 已发出确认之后接纳，必须等待下一轮。
	driver.checkReads(ctx)
	require.Equal(t, firstRound, driver.readRequest)
	require.Zero(t, driver.reads[1].round)
	probe, err := DecodeRaftPacket(ctx.system.mailbox.envelopes[0].Message().(*RaftWire).Data)
	require.NoError(t, err)
	out, err := network.nodes[probe.To].Step(probe)
	require.NoError(t, err)
	_, err = driver.node.Step(out[0])
	require.NoError(t, err)
	driver.applied = 0
	driver.checkReads(ctx)
	require.Empty(t, ctx.replies, "取得多数派确认后仍须等待本地应用到确认位置")
	for _, write := range driver.readPending {
		driver.written(ctx, raftWriteDone{lifetime: driver.lifetime, write: write})
	}
	driver.applied = driver.store.Index()
	driver.checkReads(ctx)
	require.Len(t, ctx.replies, 1)
	require.NoError(t, ctx.replies[0].Err)
	require.False(t, ctx.replies[0].Exists)
	require.Equal(t, driver.node.hard.Term, ctx.replies[0].Term)
	require.Len(t, driver.reads, 1)
	require.Greater(t, driver.readRequest, firstRound)
	require.False(t, driver.node.read.ready, "后到读取不能共享旧确认")
	require.Len(t, ctx.system.mailbox.envelopes, 4)
	probe, err = DecodeRaftPacket(ctx.system.mailbox.envelopes[2].Message().(*RaftWire).Data)
	require.NoError(t, err)
	out, err = network.nodes[probe.To].Step(probe)
	require.NoError(t, err)
	_, err = driver.node.Step(out[0])
	require.NoError(t, err)
	driver.checkReads(ctx)
	require.Len(t, ctx.replies, 2)
	require.Empty(t, driver.reads)
	require.Zero(t, driver.readRequest)
	require.Nil(t, driver.node.read)
}

func TestRaftRuntimeGet_TimeoutLeadershipAndClose(t *testing.T) {
	for _, reason := range []string{"timeout", "term", "follower", "close", "io"} {
		t.Run(reason, func(t *testing.T) {
			driver, ctx, _ := newReadDriver(t, 3)
			driver.get(ctx, RaftRuntimeGet{Key: "owner"}, time.Now())
			driver.checkReads(ctx)
			want := ErrRaftNotLeader
			switch reason {
			case "timeout":
				driver.reads[0].deadline = time.Now().Add(-time.Second)
				want = ErrRaftReadTimeout
			case "term":
				driver.node.hard.Term++
			case "follower":
				driver.node.follow(0)
			case "close":
				driver.close()
				want = ErrRaftRuntimeClosed
			case "io":
				driver.fail(io.ErrUnexpectedEOF)
				want = io.ErrUnexpectedEOF
			}
			driver.checkReads(ctx)
			driver.checkReads(ctx)
			require.Len(t, ctx.replies, 1)
			require.ErrorIs(t, ctx.replies[0].Err, want)
			require.NotErrorIs(t, ctx.replies[0].Err, ErrRaftOutcomeUnknown, "读取没有命令副作用，不应使用写入结果未知语义")
			require.Empty(t, driver.reads)
			require.Zero(t, driver.readRequest)
		})
	}
}

func TestRaftRuntimeGet_ExpirationReclaimsCapacity(t *testing.T) {
	driver, ctx, _ := newReadDriver(t, 3)
	driver.opts.MaxPendingReads = 1
	driver.get(ctx, RaftRuntimeGet{Key: "owner"}, time.Now().Add(-time.Minute))
	driver.checkReads(ctx)
	require.Len(t, ctx.replies, 1)
	require.ErrorIs(t, ctx.replies[0].Err, ErrRaftReadTimeout)
	driver.get(ctx, RaftRuntimeGet{Key: "owner"}, time.Now())
	driver.checkReads(ctx)
	require.Len(t, driver.reads, 1)
	require.NotZero(t, driver.readRequest)
}

func TestRaftRuntimeGet_WaitsForCurrentTermCommit(t *testing.T) {
	driver, ctx, network := newReadDriver(t, 3)
	driver.node.follow(0)
	network.campaign(t, 1)
	network.deliver(t, 0)
	network.deliver(t, 1)
	require.Equal(t, RaftLeader, driver.node.role)
	driver.get(ctx, RaftRuntimeGet{Key: "owner"}, time.Now())
	driver.checkReads(ctx)
	require.Empty(t, ctx.replies)
	require.Empty(t, ctx.system.mailbox.envelopes, "本任期条目未提交前不发读取确认")
	require.Zero(t, driver.readRequest)
	network.drain(t)
	driver.checkReads(ctx)
	require.NotZero(t, driver.readRequest)
	require.Empty(t, ctx.replies, "提交空操作不能代替本轮读取的多数派确认")
	probe, err := DecodeRaftPacket(ctx.system.mailbox.envelopes[0].Message().(*RaftWire).Data)
	require.NoError(t, err)
	out, err := network.nodes[probe.To].Step(probe)
	require.NoError(t, err)
	_, err = driver.node.Step(out[0])
	require.NoError(t, err)
	driver.checkReads(ctx)
	require.Empty(t, ctx.replies, "仍需等本地状态机应用到确认位置")
	driver.apply(ctx)
	driver.checkReads(ctx)
	require.Len(t, ctx.replies, 1)
	require.NoError(t, ctx.replies[0].Err)
	require.Equal(t, driver.node.hard.Commit, ctx.replies[0].Index)
}

func TestRaftRuntimeGet_ExpiredRoundCannotCompleteNewRead(t *testing.T) {
	driver, ctx, network := newReadDriver(t, 3)
	driver.get(ctx, RaftRuntimeGet{Key: "owner"}, time.Now())
	driver.checkReads(ctx)
	probe, err := DecodeRaftPacket(ctx.system.mailbox.envelopes[0].Message().(*RaftWire).Data)
	require.NoError(t, err)
	out, err := network.nodes[probe.To].Step(probe)
	require.NoError(t, err)
	oldReply := out[0]
	for _, write := range driver.readPending {
		driver.written(ctx, raftWriteDone{lifetime: driver.lifetime, write: write})
	}
	driver.reads[0].deadline = time.Now().Add(-time.Second)
	driver.checkReads(ctx)
	driver.get(ctx, RaftRuntimeGet{Key: "owner"}, time.Now())
	driver.checkReads(ctx)
	require.Greater(t, driver.readRequest, oldReply.Request)
	_, err = driver.node.Step(oldReply)
	require.NoError(t, err)
	driver.checkReads(ctx)
	require.Len(t, ctx.replies, 1)
	require.ErrorIs(t, ctx.replies[0].Err, ErrRaftReadTimeout)
	require.Len(t, driver.reads, 1)
	probe, err = DecodeRaftPacket(ctx.system.mailbox.envelopes[2].Message().(*RaftWire).Data)
	require.NoError(t, err)
	out, err = network.nodes[probe.To].Step(probe)
	require.NoError(t, err)
	_, err = driver.node.Step(out[0])
	require.NoError(t, err)
	driver.checkReads(ctx)
	require.Len(t, ctx.replies, 2)
	require.NoError(t, ctx.replies[1].Err)
}

func TestRaftRuntimeGet_IndependentSendSlotsShareByteBudget(t *testing.T) {
	driver, ctx, _ := newReadDriver(t, 3)
	driver.opts.MaxPendingBytes = 2 * raftPacketHeaderBytes
	out, err := driver.node.beginRead()
	require.NoError(t, err)
	appendRequest := driver.node.message(RaftAppendRequest, 2, 1, 0, 0)
	driver.message(ctx, appendRequest, false)
	driver.message(ctx, out[0], false)
	require.Len(t, ctx.system.mailbox.envelopes, 2, "同一成员的复制与读确认可独立写出")
	require.EqualValues(t, driver.opts.MaxPendingBytes, driver.pendingBytes)
	driver.message(ctx, out[1], false)
	require.Len(t, ctx.system.mailbox.envelopes, 2, "读取与复制仍共享总字节上限")
	oldWrite := driver.readPending[2]
	driver.written(ctx, raftWriteDone{lifetime: driver.lifetime, write: oldWrite})
	require.NotNil(t, driver.pending[2], "读确认完成不能释放复制槽")
	driver.message(ctx, out[0], false)
	newWrite := driver.readPending[2]
	require.NotSame(t, oldWrite, newWrite)
	driver.written(ctx, raftWriteDone{lifetime: driver.lifetime, write: oldWrite})
	require.Same(t, newWrite, driver.readPending[2], "旧完成不能释放新写出")
	require.EqualValues(t, driver.opts.MaxPendingBytes, driver.pendingBytes)
	driver.written(ctx, raftWriteDone{lifetime: driver.lifetime, write: driver.pending[2]})
	require.Same(t, newWrite, driver.readPending[2], "复制完成不能释放读确认槽")
	driver.message(ctx, out[1], false)
	require.Len(t, ctx.system.mailbox.envelopes, 4)
}

type readTermFailureStorage struct{ raftNodeStorage }

func (readTermFailureStorage) Term(uint64) (uint64, error) { return 0, io.ErrUnexpectedEOF }

func TestRaftRuntimeGet_StorageFailureClosesRuntime(t *testing.T) {
	driver, ctx, _ := newReadDriver(t, 3)
	lifetime := driver.lifetime
	driver.node.storage = readTermFailureStorage{driver.node.storage}
	driver.get(ctx, RaftRuntimeGet{Key: "owner"}, time.Now())
	driver.checkReads(ctx)
	driver.checkReads(ctx)
	require.Len(t, ctx.replies, 1)
	require.ErrorIs(t, ctx.replies[0].Err, io.ErrUnexpectedEOF)
	require.True(t, lifetime.closed.Load())
	require.Nil(t, driver.node)
	require.Empty(t, driver.reads)
	require.Zero(t, driver.pendingBytes)
}
