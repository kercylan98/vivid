package gossip_test

import (
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_GossipDataCenterTCPRelay(t *testing.T) {
	var systems []vivid.PrimaryActorSystem
	var refs []vivid.ActorRef
	var snapshots []gossip.SnapshotResponse
	for _, dataCenter := range []string{"dc-a", "dc-b", ""} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		require.NoError(t, listener.Close())
		system := bootstrap.NewActorSystem(
			vivid.WithActorSystemRemoting(address),
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		)
		require.NoError(t, system.Start())
		t.Cleanup(func() { require.NoError(t, system.Stop(3*time.Second)) })
		member := gossip.NewSnapshotActor(vivid.NewActorSystemClusterOptions(
			vivid.WithActorSystemClusterName("vivid-test"),
			vivid.WithActorSystemClusterDataCenter(dataCenter),
			vivid.WithActorSystemClusterLaunchDelay(0),
			vivid.WithActorSystemClusterGossipInterval(time.Hour),
			vivid.WithActorSystemClusterGracefulShutdownTimeout(50*time.Millisecond),
		))
		ref, err := system.ActorOf(member)
		require.NoError(t, err)
		_, err = member.Readiness().Wait(system, time.Second).Result()
		require.NoError(t, err)
		message, err := system.Ask(ref, gossip.TestSnapshotRequest{}, time.Second).Result()
		require.NoError(t, err)
		snapshot := message.(gossip.SnapshotResponse)
		assert.Equal(t, dataCenter, snapshot.View.Members().Get(snapshot.ID).DataCenter)
		systems, refs, snapshots = append(systems, system), append(refs, ref), append(snapshots, snapshot)
	}
	// C 尚未与其他节点通信，先订阅它的首次成员变化。
	events := make(chan vivid.ClusterSnapshot, 16)
	subscribed := make(chan struct{})
	_, err := systems[2].ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.EventStream().Subscribe(ctx, ves.ClusterMembersChangedEvent{})
			close(subscribed)
		case ves.ClusterMembersChangedEvent:
			events <- message.Snapshot
		}
	}))
	require.NoError(t, err)
	select {
	case <-subscribed:
	case <-time.After(time.Second):
		t.Fatal("subscriber not ready")
	}
	origin := snapshots[0]
	ping := gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, origin.View.Members().Get(origin.ID), origin.View.Members(), origin.View.Version())
	message, err := systems[0].Ask(refs[1], ping, time.Second).Result()
	require.NoError(t, err)
	pong := message.(*gossipmessages.Pong)
	assert.Equal(t, "dc-b", pong.Info.DataCenter, "实际注册 Codec 的 Pong 保留发送方机房")
	assert.Equal(t, "dc-a", pong.MemberList.Get(origin.ID).DataCenter)
	message, err = systems[1].Ask(refs[1], gossip.TestSnapshotRequest{}, time.Second).Result()
	require.NoError(t, err)
	relay := message.(gossip.SnapshotResponse)
	// B 的全量视图把 A 的声明转发给 C，C 无需先从 A 收到该成员记录。
	ping = gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, relay.View.Members().Get(relay.ID), relay.View.Members(), relay.View.Version())
	message, err = systems[1].Ask(refs[2], ping, time.Second).Result()
	require.NoError(t, err)
	pong = message.(*gossipmessages.Pong)
	assert.Empty(t, pong.Info.DataCenter, "未声明的机房保持空值")
	assert.Equal(t, "dc-a", pong.MemberList.Get(origin.ID).DataCenter)
	assert.Equal(t, "dc-b", pong.MemberList.Get(relay.ID).DataCenter)
	select {
	case snapshot := <-events:
		originMember, found := snapshot.Member(origin.ID)
		require.True(t, found)
		assert.Equal(t, "dc-a", originMember.DataCenter, "转发声明进入成员事件")
		relayMember, found := snapshot.Member(relay.ID)
		require.True(t, found)
		assert.Equal(t, "dc-b", relayMember.DataCenter)
	case <-time.After(time.Second):
		t.Fatal("relayed member event not received")
	}
	message, err = systems[2].Ask(refs[2], gossip.TestSnapshotRequest{}, time.Second).Result()
	require.NoError(t, err)
	observed, found := message.(gossip.SnapshotResponse).Snapshot.Member(origin.ID)
	require.True(t, found)
	assert.Equal(t, "dc-a", observed.DataCenter)
}
