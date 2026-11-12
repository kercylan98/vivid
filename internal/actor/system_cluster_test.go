package actor_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clusterTestAddress(t testing.TB) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func startClusterTestSystem(t testing.TB, address string, delay time.Duration, seeds ...string) *actor.System {
	t.Helper()
	system := actor.NewSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		vivid.WithActorSystemRemoting(address),
		vivid.WithActorSystemClusterOptions(
			vivid.WithActorSystemClusterSeeds(seeds...),
			vivid.WithActorSystemClusterLaunchDelay(delay),
			vivid.WithActorSystemClusterGossipInterval(50*time.Millisecond),
			vivid.WithActorSystemClusterGracefulShutdownTimeout(time.Second),
		),
	)
	require.NoError(t, system.Start())
	t.Cleanup(func() { require.NoError(t, system.Stop(3*time.Second)) })
	_, err := system.ClusterReady(2 * time.Second).Result()
	require.NoError(t, err)
	return system
}

func TestSystem_ClusterSnapshotErrors(t *testing.T) {
	system := actor.NewSystem()
	_, err := system.ClusterSnapshot().Result()
	assert.ErrorIs(t, err, vivid.ErrorActorSystemNotStarted)
	require.NoError(t, system.Start())
	_, err = system.ClusterSnapshot().Result()
	assert.ErrorIs(t, err, vivid.ErrorClusterNotEnabled)
	require.NoError(t, system.Stop())
	_, err = system.ClusterSnapshot().Result()
	assert.ErrorIs(t, err, vivid.ErrorActorSystemStopped)

	cluster := actor.NewSystem(
		vivid.WithActorSystemRemoting(clusterTestAddress(t)),
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterLaunchDelay(300*time.Millisecond)),
	)
	require.NoError(t, cluster.Start())
	t.Cleanup(func() { require.NoError(t, cluster.Stop(3*time.Second)) })
	snapshot, err := cluster.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	local, _ := snapshot.Member(snapshot.LocalID())
	assert.Equal(t, vivid.ClusterMemberStatusNone, local.Status, "启动延迟不得阻塞快照查询")
	future := cluster.ClusterReady(time.Second)
	future.Close(context.Canceled)
	_, err = future.Result()
	assert.ErrorIs(t, err, context.Canceled)
	snapshot, err = cluster.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	assert.Equal(t, 1, snapshot.Len())
}

type clusterObserved struct {
	initial  bool
	snapshot vivid.ClusterSnapshot
}

type clusterBarrier struct{}

func TestSystem_ClusterSnapshotEvents(t *testing.T) {
	address := clusterTestAddress(t)
	system := startClusterTestSystem(t, address, 0)
	initial, err := system.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	local, found := initial.Member(initial.LocalID())
	require.True(t, found)
	assert.Equal(t, address, local.Address)
	assert.Equal(t, vivid.ClusterMemberStatusUp, local.Status)
	assert.Equal(t, uint64(2), initial.Revision())

	var subscribers []vivid.ActorRef
	var observations []chan clusterObserved
	for range 2 {
		updates := make(chan clusterObserved, 16)
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch message := ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.EventStream().Subscribe(ctx, ves.ClusterMembersChangedEvent{})
				ctx.System().ClusterSnapshot(time.Second).PipeTo(ctx.Ref().ToActorRefs())
			case *vivid.PipeResult:
				if message.Error == nil {
					updates <- clusterObserved{initial: true, snapshot: message.Message.(vivid.ClusterSnapshot)}
				}
			case ves.ClusterMembersChangedEvent:
				updates <- clusterObserved{snapshot: message.Snapshot}
			case clusterBarrier:
				ctx.Reply(true)
			}
		}))
		require.NoError(t, err)
		subscribers = append(subscribers, ref)
		observations = append(observations, updates)
		select {
		case observation := <-updates:
			require.True(t, observation.initial)
			assert.Equal(t, initial.Revision(), observation.snapshot.Revision())
		case <-time.After(time.Second):
			t.Fatal("initial snapshot was not delivered")
		}
	}
	peer := startClusterTestSystem(t, clusterTestAddress(t), 0, address)
	peerSnapshot, err := peer.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		snapshot, err := system.ClusterSnapshot(time.Second).Result()
		if err != nil {
			return false
		}
		member, found := snapshot.Member(peerSnapshot.LocalID())
		return found && member.Status == vivid.ClusterMemberStatusUp
	}, 3*time.Second, 20*time.Millisecond)
	for i, ref := range subscribers {
		require.NoError(t, system.Ask(ref, clusterBarrier{}, time.Second).Wait())
		var events []vivid.ClusterSnapshot
		for len(observations[i]) > 0 {
			observation := <-observations[i]
			require.False(t, observation.initial)
			events = append(events, observation.snapshot)
		}
		require.Len(t, events, 2, "加入和 Up 各产生一次事件，重复周期同步不产生事件")
		for j, snapshot := range events {
			assert.Equal(t, initial.LocalID(), snapshot.LocalID())
			assert.Equal(t, initial.Revision()+uint64(j)+1, snapshot.Revision())
			assert.Equal(t, 2, snapshot.Len())
			members := snapshot.Members()
			members[0].Address = "subscriber mutation"
			assert.NotEqual(t, members, snapshot.Members())
		}
		joining, _ := events[0].Member(peerSnapshot.LocalID())
		up, _ := events[1].Member(peerSnapshot.LocalID())
		assert.Equal(t, vivid.ClusterMemberStatusJoining, joining.Status)
		assert.Equal(t, vivid.ClusterMemberStatusUp, up.Status)
	}
	assert.Equal(t, 1, initial.Len(), "旧快照不得随成员变化")
}

func TestSystem_ClusterSnapshotSubscriptionWindow(t *testing.T) {
	address := clusterTestAddress(t)
	system := startClusterTestSystem(t, address, 0)
	baseline, err := system.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	subscribed := make(chan struct{})
	query := make(chan struct{})
	completed := make(chan uint64, 1)
	// 发生变更时订阅者仍在处理 OnLaunch，以验证订阅后、首次查询前的窗口。
	_, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.EventStream().Subscribe(ctx, ves.ClusterMembersChangedEvent{})
			close(subscribed)
			<-query
			ctx.System().ClusterSnapshot(time.Second).PipeTo(ctx.Ref().ToActorRefs())
		case ves.ClusterMembersChangedEvent:
			// 此时用户处理函数尚未完成，Gossip 查询仍应正常应答。
		case *vivid.PipeResult:
			if message.Error == nil {
				completed <- message.Message.(vivid.ClusterSnapshot).Revision()
			}
		}
	}))
	require.NoError(t, err)
	// 即使测试失败也先释放订阅者，再关闭系统。
	released := false
	defer func() {
		if !released {
			close(query)
		}
	}()
	select {
	case <-subscribed:
	case <-time.After(time.Second):
		t.Fatal("subscriber did not start")
	}
	peer := startClusterTestSystem(t, clusterTestAddress(t), 0, address)
	peerSnapshot, err := peer.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	var latest vivid.ClusterSnapshot
	require.Eventually(t, func() bool {
		latest, err = system.ClusterSnapshot(time.Second).Result()
		if err != nil {
			return false
		}
		member, found := latest.Member(peerSnapshot.LocalID())
		return found && member.Status == vivid.ClusterMemberStatusUp
	}, 3*time.Second, 20*time.Millisecond)
	assert.Greater(t, latest.Revision(), baseline.Revision(), "阻塞的订阅者不能阻止视图更新或查询")
	close(query)
	released = true
	select {
	case revision := <-completed:
		assert.Equal(t, latest.Revision(), revision, "首次查询应包含订阅窗口内已接受的变更")
	case <-time.After(time.Second):
		t.Fatal("snapshot was not delivered")
	}
}

func BenchmarkSystem_ClusterSnapshot(b *testing.B) {
	system := startClusterTestSystem(b, clusterTestAddress(b), 0)
	_, err := system.ClusterSnapshot(time.Second).Result()
	require.NoError(b, err)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := system.ClusterSnapshot(time.Second).Result(); err != nil {
			b.Fatal(err)
		}
	}
}
