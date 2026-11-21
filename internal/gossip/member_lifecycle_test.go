package gossip

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_GossipMemberLifecycle(t *testing.T) {
	statuses := []endpoint.Status{endpoint.StatusJoining, endpoint.StatusUp, endpoint.StatusLeaving, endpoint.StatusExiting, endpoint.StatusRemoved}
	for _, protocol := range []string{"ping", "pong"} {
		for _, direct := range []bool{false, true} {
			for _, before := range statuses {
				for _, after := range statuses {
					name := protocol + "/" + map[bool]string{true: "direct", false: "relayed"}[direct] + "/" + before.String() + "_to_" + after.String()
					t.Run(name, func(t *testing.T) {
						a, b, ctx, _ := newPongAckActors()
						known := b.info.Clone()
						known.Status = before
						a.view.Members().Upsert(known)
						original, outbound := a.currentSnapshot(), a.currentOutboundView()
						incoming := known.Clone()
						incoming.Status = after
						members := memberlist.New(nil, *incoming)
						version := a.view.Version().Clone()
						version.Update(incoming.ID(), 2)
						sender := a.info.Clone()
						if direct {
							sender = incoming
						}
						if protocol == "ping" {
							a.onPing(ctx, gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, sender, members, version))
						} else {
							a.onPong(ctx, gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, sender, members, version))
						}
						if after < before {
							assert.Equal(t, known, a.view.Members().Get(known.ID()))
							assert.Equal(t, uint64(1), a.view.Version().Get(known.ID()), "回退记录的高版本不能导入")
							assert.Same(t, original, a.currentSnapshot())
							assert.Same(t, outbound, a.currentOutboundView())
							assert.Nil(t, ctx.stream, "拒绝回退不能发布成员或可达性事件")
							if direct {
								assert.Nil(t, ctx.reply, "直接发送方回退时丢弃整条消息")
							}
						} else {
							assert.Equal(t, incoming, a.view.Members().Get(incoming.ID()))
							assert.Equal(t, uint64(2), a.view.Version().Get(incoming.ID()), "允许相同状态和跳过中间阶段的传播")
							require.NotNil(t, ctx.stream)
							assert.Len(t, ctx.stream.events, 1)
							assert.NotSame(t, outbound, a.currentOutboundView())
						}
						old, exists := original.Member(known.ID())
						require.True(t, exists)
						assert.Equal(t, before, old.Status)
						assert.Equal(t, uint64(1), old.Revision, "旧快照不随合并结果变化")
					})
				}
			}
		}
	}
}

func TestActor_GossipMemberLifecycleRecovery(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(map[bool]string{true: "direct", false: "relayed"}[direct], func(t *testing.T) {
			a, b, ctx, _ := newPongAckActors()
			known := b.info.Clone()
			known.Status = endpoint.StatusLeaving
			a.view.Members().Upsert(known)
			now := time.Now()
			sequence := a.reachability.begin(known.ActorRef, a.view.Members(), now, time.Hour, 1)
			original, outbound := a.currentSnapshot(), a.currentOutboundView()
			bad := known.Clone()
			bad.Status = endpoint.StatusJoining
			version := a.view.Version().Clone()
			version.Update(bad.ID(), 100)
			sender := a.info.Clone()
			if direct {
				sender = bad
			}
			pong := gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, sender, memberlist.New(nil, *bad), version)
			pong.ProbeSequence, pong.ProbeOriginID = sequence, a.info.ID()
			a.onPong(ctx, pong)
			assert.Same(t, original, a.currentSnapshot())
			assert.Same(t, outbound, a.currentOutboundView())
			assert.Nil(t, ctx.stream)
			assert.Equal(t, sequence, a.reachability.pending[known.ActorRef.GetAddress()].sequence)
			assert.Equal(t, vivid.ClusterReachabilityUnknown, a.reachability.state(known.ID()), "回退应答不能完成观察")
			good := known.Clone()
			good.Status = endpoint.StatusExiting
			version = a.view.Version().Clone()
			version.Update(good.ID(), 2)
			pong = gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, good, memberlist.New(nil, *good), version)
			pong.ProbeSequence, pong.ProbeOriginID = sequence, a.info.ID()
			a.onPong(ctx, pong)
			assert.Equal(t, endpoint.StatusExiting, a.view.Members().Get(good.ID()).Status)
			assert.Equal(t, uint64(2), a.view.Version().Get(good.ID()), "被拒绝的版本 100 不能阻断合法版本 2")
			assert.Empty(t, a.reachability.pending)
			assert.Equal(t, vivid.ClusterReachabilityReachable, a.reachability.state(good.ID()))
		})
	}
}

func TestActor_GossipMemberLifecycleRestart(t *testing.T) {
	a, b, ctx, _ := newPongAckActors()
	removed := b.info.Clone()
	removed.Status = endpoint.StatusRemoved
	a.view.Members().Upsert(removed)
	restarted := endpoint.NewInformation(removed.ActorRef.Clone(), removed.CreatedAt.Add(-time.Second))
	restarted.Status, restarted.DataCenter = endpoint.StatusUp, "dc-c"
	version := a.view.Version().Clone()
	version.Update(restarted.ID(), 1)
	a.onPing(ctx, gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, restarted, memberlist.New(nil, *restarted), version))
	assert.Equal(t, restarted, a.view.Members().Get(restarted.ID()), "同地址新实例仍可用独立身份加入")
	assert.Equal(t, removed, a.view.Members().Get(removed.ID()))
	assert.Equal(t, uint64(1), a.view.Version().Get(removed.ID()))
}
