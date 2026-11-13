package gossip

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_GossipClusterNameBoundary(t *testing.T) {
	for _, name := range []string{"", "other", "Vivid-test"} {
		for _, reply := range []bool{false, true} {
			messageType := "/ping"
			if reply {
				messageType = "/pong"
			}
			t.Run(name+messageType, func(t *testing.T) {
				local, remote, relayed := reachableMember("local:10001"), reachableMember("peer:10002"), reachableMember("relayed:10003")
				a := &Actor{info: local, view: NewClusterView(log.GetDefault()), opts: vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")), reachability: newLocalReachability()}
				a.view.Members().Upsert(local)
				a.view.Version().Update(local.ID(), 2)
				ctx := &reachabilityContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}, sender: remote.ActorRef}
				before := a.currentSnapshot()
				sequence := a.reachability.begin(remote.ActorRef, nil, time.Now(), time.Hour, 1)
				pending := a.reachability.pending[remote.ActorRef.GetAddress()]
				members, version := a.view.Members().Clone(), a.view.Version().Clone()
				for _, member := range []*endpoint.Information{remote, relayed} {
					members.Upsert(member)
					version.Update(member.ID(), 2)
				}
				if reply {
					pong := gossipmessages.NewPong(name, gossipmessages.GossipTypeCycleSYNC, remote, members, version)
					pong.ProbeSequence, pong.ProbeOriginID = sequence, local.ID()
					a.onPong(ctx, pong)
				} else {
					ping := gossipmessages.NewPing(name, gossipmessages.GossipTypeCycleSYNC, remote, members, version)
					ping.ProbeSequence = sequence
					a.onPing(ctx, ping)
				}
				assert.Same(t, before, a.currentSnapshot(), "不匹配的直接和转发视图都不能改变本地快照")
				assert.Len(t, a.view.Members().List(), 1)
				assert.Zero(t, a.view.Version().Get(remote.ID()))
				assert.Zero(t, a.view.Version().Get(relayed.ID()))
				assert.Equal(t, pending, a.reachability.pending[remote.ActorRef.GetAddress()])
				assert.Empty(t, a.reachability.observations)
				assert.Nil(t, ctx.stream, "拒绝时不发布成员或可达性事件")
				assert.Nil(t, ctx.reply, "周期消息不向其他集群返回成员视图")
				assert.Empty(t, ctx.sent)
				// 同一请求的有效响应仍能完成观察；拒绝不能提前消费序号。
				pong := gossipmessages.NewPong(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, remote, members, version)
				pong.ProbeSequence, pong.ProbeOriginID = sequence, local.ID()
				a.onPong(ctx, pong)
				assert.Len(t, a.view.Members().List(), 3)
				assert.Equal(t, vivid.ClusterReachabilityReachable, a.reachability.state(remote.ID()))
			})
		}
	}
}

func TestActor_GossipClusterNameRejection(t *testing.T) {
	local, joiner := reachableMember("local:10001"), reachableMember("joiner:10002")
	a := &Actor{info: local, view: NewClusterView(log.GetDefault()), opts: vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("production"))}
	a.view.Members().Upsert(local)
	a.view.Version().Update(local.ID(), 2)
	ctx := &reachabilityContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}, sender: joiner.ActorRef}
	ping := gossipmessages.NewPing("test", gossipmessages.GossipTypeJoin, joiner, a.view.Members().Clone(), a.view.Version().Clone())
	a.onPing(ctx, ping)
	rejected, ok := ctx.reply.(*gossipmessages.JoinRejected)
	require.True(t, ok)
	assert.Equal(t, "production", rejected.ClusterName)
	assert.Len(t, a.view.Members().List(), 1)
	// 拒绝消息仅传输名称，不包含成员或版本数据。
	w := serialization.NewWriter().Write(rejected)
	require.NoError(t, w.Err())
	var decoded gossipmessages.JoinRejected
	require.NoError(t, serialization.NewReader(w.Bytes()).Read(&decoded))
	assert.Equal(t, *rejected, decoded)
}
