package actor

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

type controlEnvelopHandler struct {
	remoting.NetworkEnvelopHandler
	hold func(probeRequest) bool
}

func (h *controlEnvelopHandler) HandleRemotingEnvelop(system bool, sender, receiver string, message any) error {
	request := probeRequest{system, sender, receiver, message}
	if h.hold != nil && h.hold(request) {
		return nil
	}
	return h.NetworkEnvelopHandler.HandleRemotingEnvelop(system, sender, receiver, message)
}

// newRemoteControlSystem 使用第四个真实 TCP 系统；同样通过代理参与分区，不借用 Raft 节点的本地入口。
func newRemoteControlSystem(t *testing.T, n *raftRuntimeNetwork, hold func(probeRequest) bool, prepare ...func(*System)) *System {
	t.Helper()
	return newRemoteControlSystemAt(t, n, 4, hold, prepare...)
}

func newRemoteControlSystemAt(t *testing.T, n *raftRuntimeNetwork, id uint64, hold func(probeRequest) bool, prepare ...func(*System)) *System {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	backend := admissionSystemAddress(t)
	n.mu.Lock()
	n.addresses[listener.Addr().String()] = id
	n.listeners = append(n.listeners, listener)
	n.mu.Unlock()
	n.wg.Add(1)
	go n.proxy(listener, backend, id)
	system := NewSystem(vivid.WithActorSystemRemoting(backend, listener.Addr().String()),
		vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnect(2, 20*time.Millisecond, 40*time.Millisecond, 2, false)),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))))
	system.remotingEnvelopHandler = &controlEnvelopHandler{NetworkEnvelopHandler: system.remotingEnvelopHandler, hold: hold}
	for _, configure := range prepare {
		configure(system)
	}
	require.NoError(t, system.Start())
	t.Cleanup(func() {
		if !system.requestsStopped() {
			require.NoError(t, system.Stop(3*time.Second))
		}
	})
	return system
}

func remoteOwnershipClient(t *testing.T, system *System, n *raftRuntimeNetwork) *virtualOwnershipClient {
	t.Helper()
	owner, err := clusterstore.NewVirtualOwner(system.Ref().GetAddress())
	require.NoError(t, err)
	client, err := newVirtualOwnershipClient(system, clusterstore.RaftConfig{GroupID: "runtime", NodeID: 1, Voters: []uint64{1, 2, 3}}, n.refs, owner)
	require.NoError(t, err)
	return client
}

func TestRaftControl_RemoteOwnershipSurvivesLeaderChange(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	old := n.leader(t, 1, 2, 3)
	system := newRemoteControlSystem(t, n, nil)
	client := remoteOwnershipClient(t, system, n)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, err := client.claim(ctx, virtualKey{"worker", "one"})
	require.NoError(t, err)
	require.NoError(t, client.validate(ctx, first))
	require.Equal(t, old, client.control.leader.Load())
	n.partition(old)
	var majority []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != old {
			majority = append(majority, id)
		}
	}
	leader := n.leader(t, majority...)
	// 已缓存旧 Leader 的客户端先定位，旧入口不可达时可安全联络其他成员。
	require.NoError(t, client.validate(ctx, first))
	require.Equal(t, leader, client.control.leader.Load())
	require.NoError(t, client.release(ctx, first))
	next, err := client.claim(ctx, virtualKey{"worker", "one"})
	require.NoError(t, err)
	require.Greater(t, next.Generation, first.Generation)
	require.ErrorIs(t, client.release(ctx, first), clusterstore.ErrVirtualOwnershipConflict)
	require.NoError(t, client.validate(ctx, next))
	require.NoError(t, client.release(ctx, next))
	n.partition()
}

func TestRaftControl_RemoteLostWriteReplyIsNotRetried(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	replies := make(chan probeRequest, 2)
	var writes atomic.Int32
	system := newRemoteControlSystem(t, n, func(request probeRequest) bool {
		if response, ok := request.message.(*clusterstore.RaftControlResponse); ok && response.Operation == clusterstore.RaftControlExecute {
			writes.Add(1)
			replies <- request
			return true
		}
		return false
	})
	client := remoteOwnershipClient(t, system, n)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		ownership, err := client.claim(ctx, virtualKey{"worker", "one"})
		if ownership != (clusterstore.VirtualOwnership{}) {
			result <- clusterstore.ErrInvalidVirtualOwnership
			return
		}
		result <- err
	}()
	late := receiveRetryResult(t, replies)
	response := late.message.(*clusterstore.RaftControlResponse)
	require.NoError(t, response.Err())
	require.True(t, response.Applied, "远端已提交，但客户端尚未收到回执")
	require.ErrorIs(t, receiveRetryResult(t, result), clusterstore.ErrRaftOutcomeUnknown)
	require.EqualValues(t, 1, writes.Load())
	n.committed(t, response.Index, 1, 2, 3)
	require.Equal(t, response.Index, n.status(t, leader).LastIndex, "丢失回执后不能换节点重试 CAS")
	// 放行真实迟到回执；已结束等待不能因此创建业务实例或重新执行写入。
	handler := system.remotingEnvelopHandler.(*controlEnvelopHandler)
	require.NoError(t, handler.NetworkEnvelopHandler.HandleRemotingEnvelop(late.system, late.sender, late.receiver, late.message))
	current, exists := n.ownership(t, leader, "worker", "one")
	require.True(t, exists)
	require.Equal(t, response.Index, current.Generation)
	require.NoError(t, client.validate(context.Background(), current))
	require.EqualValues(t, 1, writes.Load())
}

func TestRaftControl_RemoteVirtualFailsClosedAndRecovers(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	n.leader(t, 1, 2, 3)
	var providers atomic.Int32
	system := newRemoteControlSystem(t, n, nil, func(system *System) {
		owner, err := clusterstore.NewVirtualOwner(system.options.RemotingAdvertiseAddress)
		require.NoError(t, err)
		client, err := newVirtualOwnershipClient(system, clusterstore.RaftConfig{GroupID: "runtime", NodeID: 1, Voters: []uint64{1, 2, 3}}, n.refs, owner)
		require.NoError(t, err)
		require.NoError(t, system.virtualCoordinator.configureOwnership(client, 120*time.Millisecond))
		system.options.VirtualActorProviders = map[string]vivid.VirtualActorProvider{"worker": vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			providers.Add(1)
			return vivid.ActorFN(virtualReply)
		})}
	})
	n.partition(4)
	ref := system.VirtualRef("worker", "one")
	require.Error(t, system.Ask(ref, 1).Wait())
	require.Zero(t, providers.Load(), "无法联络协调组时不能调用业务 Provider")
	n.partition()
	require.Zero(t, providers.Load(), "恢复网络不自动重试先前失败的激活")
	require.NoError(t, system.Ask(ref, 2).Wait(), "显式新请求可重新联络并激活")
	require.EqualValues(t, 1, providers.Load())
	require.NoError(t, system.Stop(2*time.Second))
}

func TestRaftControl_RejectsDifferentGroup(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	system := newRemoteControlSystem(t, n, nil)
	client, err := newRaftControlClient(system, clusterstore.RaftConfig{GroupID: "another", NodeID: 1, Voters: []uint64{1, 2, 3}}, n.refs)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = client.call(ctx, clusterstore.RaftControlExecute, clusterstore.Command{Operation: clusterstore.Put, Key: "wrong-group"}, "")
	require.ErrorIs(t, err, clusterstore.ErrInvalidRaftControl)
	require.False(t, n.get(t, leader, "wrong-group").Exists)
}

func TestRaftControl_StaleLeaderHintRedirectsOnlyBeforeProposal(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	old := n.leader(t, 1, 2, 3)
	locations := make(chan probeRequest, 1)
	var held atomic.Bool
	system := newRemoteControlSystem(t, n, func(request probeRequest) bool {
		if response, ok := request.message.(*clusterstore.RaftControlResponse); ok && response.Operation == clusterstore.RaftControlLocate && response.Role == clusterstore.RaftLeader && held.CompareAndSwap(false, true) {
			locations <- request
			return true
		}
		return false
	})
	client := remoteOwnershipClient(t, system, n)
	client.control.leader.Store(old)
	// 定位回执被刻意保留到任期切换后，不让联络预算先于交错演练结束。
	client.control.contact = 4 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := client.claim(ctx, virtualKey{"worker", "stale"})
		result <- err
	}()
	late := receiveRetryResult(t, locations)
	n.partition(old)
	var majority []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != old {
			majority = append(majority, id)
		}
	}
	leader := n.leader(t, majority...)
	n.partition()
	require.Eventually(t, func() bool {
		status := n.status(t, old)
		return status.Role == clusterstore.RaftFollower && status.Leader == leader
	}, 2*time.Second, 10*time.Millisecond)
	handler := system.remotingEnvelopHandler.(*controlEnvelopHandler)
	require.NoError(t, handler.NetworkEnvelopHandler.HandleRemotingEnvelop(late.system, late.sender, late.receiver, late.message))
	require.NoError(t, receiveRetryResult(t, result), "旧定位不能授权写入，但明确的提案前拒绝可以安全更换入口")
	require.Equal(t, leader, client.control.leader.Load())
	current, exists := n.ownership(t, leader, "worker", "stale")
	require.True(t, exists)
	require.NoError(t, client.release(context.Background(), current))
}

func TestRaftControl_RemoteMinorityTimeoutAndStop(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	leader := n.leader(t, 1, 2, 3)
	base := n.execute(t, leader, clusterstore.Command{Operation: clusterstore.Put, Key: "base", Value: []byte("committed")})
	require.NoError(t, base.Err)
	n.committed(t, base.Index, 1, 2, 3)
	system := newRemoteControlSystem(t, n, nil)
	client := remoteOwnershipClient(t, system, n)
	var isolated []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != leader {
			isolated = append(isolated, id)
		}
	}
	n.partition(isolated...)
	// 客户端仍可联系旧 Leader，但它无法联系多数派。外层等待留足时间以观察远端错误码。
	write := system.Ask(n.refs[leader], &clusterstore.RaftControlRequest{Group: client.control.group, Node: leader, Operation: clusterstore.RaftControlExecute, Timeout: 80 * time.Millisecond, Command: clusterstore.Command{Operation: clusterstore.Put, Key: "timed"}}, 2*time.Second)
	read := system.Ask(n.refs[leader], &clusterstore.RaftControlRequest{Group: client.control.group, Node: leader, Operation: clusterstore.RaftControlGet, Timeout: 80 * time.Millisecond, Key: "base"}, 2*time.Second)
	pending := system.Ask(n.refs[leader], &clusterstore.RaftControlRequest{Group: client.control.group, Node: leader, Operation: clusterstore.RaftControlExecute, Timeout: time.Minute, Command: clusterstore.Command{Operation: clusterstore.Put, Key: "pending"}}, 2*time.Second)
	result, err := write.Result()
	require.NoError(t, err)
	response := result.(*clusterstore.RaftControlResponse)
	require.ErrorIs(t, response.Execution().Err, clusterstore.ErrRaftOutcomeUnknown)
	require.Greater(t, response.Index, base.Index)
	result, err = read.Result()
	require.NoError(t, err)
	require.ErrorIs(t, result.(*clusterstore.RaftControlResponse).Record().Err, clusterstore.ErrRaftReadTimeout)
	n.systems[leader].Kill(n.refs[leader], false, "remote pending control stop")
	result, err = pending.Result()
	require.NoError(t, err)
	require.ErrorIs(t, result.(*clusterstore.RaftControlResponse).Execution().Err, clusterstore.ErrRaftOutcomeUnknown)
	n.partition()
}
