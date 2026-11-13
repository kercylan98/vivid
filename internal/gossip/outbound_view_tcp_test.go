package gossip_test

import (
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_GossipOutboundViewDelayedEncoding(t *testing.T) {
	var systems []vivid.PrimaryActorSystem
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
		systems = append(systems, system)
	}
	member := gossip.New(vivid.NewActorSystemClusterOptions(
		vivid.WithActorSystemClusterName("vivid-test"),
		vivid.WithActorSystemClusterDataCenter("dc-a"),
		vivid.WithActorSystemClusterLaunchDelay(0),
		vivid.WithActorSystemClusterGossipInterval(time.Hour),
		vivid.WithActorSystemClusterGracefulShutdownTimeout(50*time.Millisecond),
	))
	ref, err := systems[0].ActorOf(member)
	require.NoError(t, err)
	_, err = member.Readiness().Wait(systems[0], time.Second).Result()
	require.NoError(t, err)
	echo, err := systems[1].ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if pong, ok := ctx.Message().(*gossipmessages.Pong); ok {
			ctx.Reply(pong)
		}
	}))
	require.NoError(t, err)
	remote := endpoint.NewInformation(echo, time.Unix(1, 0))
	remote.DataCenter, remote.Status = "dc-b", endpoint.StatusUp
	members, version := memberlist.New(log.GetDefault()), versionvector.New()
	members.Upsert(remote)
	version.Update(remote.ID(), 1)
	ping := gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, remote, members, version)
	ping.ProbeSequence = 17
	message, err := systems[0].Ask(ref, ping, time.Second).Result()
	require.NoError(t, err)
	first := message.(*gossipmessages.Pong)
	// 本地请求保留原始发送对象，直到后续成员合并后才交给真实远程编码。
	ping = gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeJoin, remote, members, version)
	ping.ProbeSequence = 19
	message, err = systems[0].Ask(ref, ping, time.Second).Result()
	require.NoError(t, err)
	second := message.(*gossipmessages.Pong)
	assert.Same(t, first.MemberList, second.MemberList, "不同请求共享未变的发送视图")
	assert.Equal(t, uint64(17), first.ProbeSequence)
	assert.Equal(t, uint64(19), second.ProbeSequence)
	changed := remote.Clone()
	changed.Status = endpoint.StatusRemoved
	nextMembers, nextVersion := memberlist.New(log.GetDefault()), versionvector.New()
	nextMembers.Upsert(changed)
	nextVersion.Update(changed.ID(), 2)
	message, err = systems[0].Ask(ref, gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, changed, nextMembers, nextVersion), time.Second).Result()
	require.NoError(t, err)
	current := message.(*gossipmessages.Pong)
	assert.Equal(t, endpoint.StatusRemoved, current.MemberList.Get(remote.ID()).Status)
	assert.Equal(t, uint64(2), current.Version.Get(remote.ID()))
	assert.NotSame(t, first.MemberList, current.MemberList)
	var requests []vivid.Future[vivid.Message]
	for range 8 {
		for _, retained := range []*gossipmessages.Pong{first, second} {
			requests = append(requests, systems[0].Ask(echo, retained, 3*time.Second))
		}
	}
	for index, request := range requests {
		message, err = request.Result()
		require.NoError(t, err)
		decoded := message.(*gossipmessages.Pong)
		expected := []*gossipmessages.Pong{first, second}[index%2]
		assert.Equal(t, expected.Info.ID(), decoded.Info.ID())
		assert.Equal(t, "dc-a", decoded.Info.DataCenter)
		assert.Equal(t, expected.Timestamp.UnixNano(), decoded.Timestamp.UnixNano())
		assert.Equal(t, expected.GossipType, decoded.GossipType)
		assert.Equal(t, expected.ProbeSequence, decoded.ProbeSequence)
		assert.Equal(t, remote.ID(), decoded.ProbeOriginID)
		assert.Equal(t, endpoint.StatusUp, decoded.MemberList.Get(remote.ID()).Status, "真实注册 Codec 编码的旧消息保留旧状态")
		assert.Equal(t, "dc-b", decoded.MemberList.Get(remote.ID()).DataCenter)
		assert.Equal(t, uint64(1), decoded.Version.Get(remote.ID()))
	}
}
