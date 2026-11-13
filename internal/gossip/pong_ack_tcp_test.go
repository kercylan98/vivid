package gossip_test

import (
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_GossipPongAckTCP(t *testing.T) {
	var systems []vivid.PrimaryActorSystem
	var refs []vivid.ActorRef
	for range 2 {
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
			vivid.WithActorSystemClusterLaunchDelay(0),
			vivid.WithActorSystemClusterGossipInterval(time.Hour),
			vivid.WithActorSystemClusterGracefulShutdownTimeout(50*time.Millisecond),
		))
		ref, err := system.ActorOf(member)
		require.NoError(t, err)
		_, err = member.Readiness().Wait(system, time.Second).Result()
		require.NoError(t, err)
		systems, refs = append(systems, system), append(refs, ref)
	}
	read := func(index int) gossip.SnapshotResponse {
		message, err := systems[index].Ask(refs[index], gossip.TestSnapshotRequest{}, time.Second).Result()
		require.NoError(t, err)
		return message.(gossip.SnapshotResponse)
	}
	origin := read(0)
	request := gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, origin.View.Members().Get(origin.ID), origin.View.Members(), origin.View.Version())
	message, err := systems[0].Ask(refs[1], request, time.Second).Result()
	require.NoError(t, err)
	full, ok := message.(*gossipmessages.Pong)
	require.True(t, ok, "视图不完整时返回完整视图，真实 TCP 保留版本与成员")
	assert.Equal(t, endpoint.StatusUp, full.Info.Status)
	// 让两个实际 Actor 完成一次全量同步，周期发送随后携带相同视图。
	systems[0].Tell(refs[0], full)
	before := read(0).PongAckCount
	systems[0].Tell(refs[0], gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC))
	require.Eventually(t, func() bool {
		snapshot := read(0)
		member, found := snapshot.Snapshot.Member(full.Info.ID())
		return snapshot.PongAckCount > before && found && member.Reachability == vivid.ClusterReachabilityReachable
	}, 3*time.Second, 10*time.Millisecond, "轻量应答须通过真实注册 Codec 并完成匹配的直接观察")
	request = gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeJoin, origin.View.Members().Get(origin.ID), origin.View.Members(), origin.View.Version())
	message, err = systems[0].Ask(refs[1], request, time.Second).Result()
	require.NoError(t, err)
	_, ok = message.(*gossipmessages.Pong)
	assert.True(t, ok, "初始加入始终使用完整 Pong")
}
