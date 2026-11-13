package gossip

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_GossipOutboundViewReuse(t *testing.T) {
	a, _ := newProbeBudgetActor(3, 1, true)
	first := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
	first.ProbeSequence = 17
	second := a.preparePing(gossipmessages.GossipTypeJoin)
	pong := a.preparePong(gossipmessages.GossipTypeDiscoverNewNode)
	assert.NotSame(t, first, second, "每条消息独立保留请求头")
	assert.Same(t, first.Info, second.Info)
	assert.Same(t, first.MemberList, second.MemberList)
	assert.Same(t, first.Version, second.Version)
	assert.Same(t, first.Info, pong.Info)
	assert.Same(t, first.MemberList, pong.MemberList)
	assert.Same(t, first.Version, pong.Version)
	assert.NotSame(t, a.info, first.Info)
	assert.NotSame(t, a.view.Members(), first.MemberList)
	assert.NotSame(t, a.view.Version(), first.Version)
	for _, member := range first.MemberList.List() {
		original := a.view.Members().Get(member.ID())
		assert.NotSame(t, original, member)
		assert.NotSame(t, original.ActorRef, member.ActorRef)
	}
	assert.Equal(t, uint64(17), first.ProbeSequence)
	assert.Zero(t, second.ProbeSequence)
	assert.Zero(t, pong.ProbeSequence)
	assert.Empty(t, pong.ProbeOriginID)
	assert.Equal(t, gossipmessages.GossipTypeCycleSYNC, first.GossipType)
	assert.Equal(t, gossipmessages.GossipTypeJoin, second.GossipType)
	assert.Equal(t, gossipmessages.GossipTypeDiscoverNewNode, pong.GossipType)
	assert.False(t, second.Timestamp.Before(first.Timestamp))
	assert.False(t, pong.Timestamp.Before(second.Timestamp))
}

func TestActor_GossipOutboundViewMerge(t *testing.T) {
	for _, name := range []string{"added", "version", "removed", "same_address_restart", "duplicate", "stale", "local", "conflicting_data_center"} {
		t.Run(name, func(t *testing.T) {
			a, _ := newViewStableActor(endpoint.StatusUp)
			remote := reachableMember("remote:10002")
			remote.DataCenter = "dc-a"
			a.view.Members().Upsert(remote)
			a.view.Version().Update(remote.ID(), 2)
			before := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
			candidate, revision, changed := remote.Clone(), uint64(3), true
			switch name {
			case "added":
				candidate = reachableMember("other:10003")
			case "removed":
				candidate.Status = endpoint.StatusRemoved
			case "same_address_restart":
				candidate = reachableMember(remote.ActorRef.GetAddress())
			case "duplicate":
				revision, changed = 2, false
			case "stale":
				revision, changed = 1, false
			case "local":
				candidate, changed = a.info.Clone(), false
			case "conflicting_data_center":
				candidate.DataCenter, changed = "dc-b", false
			}
			version := versionvector.New()
			version.Update(candidate.ID(), revision)
			a.mergeMember(candidate, version)
			// 合并后、事件发布前准备的消息也必须使用最新视图。
			after := a.preparePong(gossipmessages.GossipTypeCycleSYNC)
			if changed {
				assert.NotSame(t, before.MemberList, after.MemberList)
				assert.NotSame(t, before.Version, after.Version)
				assert.Equal(t, candidate.Status, after.MemberList.Get(candidate.ID()).Status)
				assert.Equal(t, revision, after.Version.Get(candidate.ID()))
			} else {
				assert.Same(t, before.MemberList, after.MemberList, "未接收的记录不重建缓存")
				assert.Same(t, before.Version, after.Version)
			}
			assert.Equal(t, endpoint.StatusUp, before.MemberList.Get(remote.ID()).Status)
			assert.Equal(t, uint64(2), before.Version.Get(remote.ID()), "旧消息的版本 map 不随合并更新")
			assert.Len(t, before.MemberList.List(), 2)
		})
	}
}

func TestActor_GossipOutboundViewLifecycle(t *testing.T) {
	for _, initial := range []endpoint.Status{endpoint.StatusNone, endpoint.StatusJoining} {
		t.Run(initial.String(), func(t *testing.T) {
			a, ctx := newViewStableActor(initial)
			first := EventBootstrap
			if initial == endpoint.StatusJoining {
				first = EventUp
			}
			for _, event := range []Event{first, EventLeaving, EventExiting, EventRemoved} {
				before := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
				status, revision := a.info.Status, a.view.Version().Get(a.info.ID())
				require.True(t, changeStatus(ctx, a, event))
				after := a.preparePong(gossipmessages.GossipTypeCycleSYNC)
				assert.NotSame(t, before.MemberList, after.MemberList)
				assert.Equal(t, a.info.Status, after.Info.Status)
				assert.Equal(t, a.info.Status, after.MemberList.Get(a.info.ID()).Status)
				assert.Equal(t, revision+1, after.Version.Get(a.info.ID()))
				assert.Equal(t, status, before.Info.Status)
				assert.Equal(t, status, before.MemberList.Get(a.info.ID()).Status)
				assert.Equal(t, revision, before.Version.Get(a.info.ID()))
			}
			before := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
			require.False(t, changeStatus(ctx, a, EventUp))
			assert.Same(t, before.MemberList, a.preparePong(gossipmessages.GossipTypeCycleSYNC).MemberList)
		})
	}
}

func TestActor_GossipOutboundViewReachability(t *testing.T) {
	a, _ := newViewStableActor(endpoint.StatusUp)
	a.reachability = newLocalReachability()
	remote := reachableMember("remote:10002")
	a.view.Members().Upsert(remote)
	a.view.Version().Update(remote.ID(), 1)
	ctx := &reachabilityContext{tombstoneContext: &tombstoneContext{ref: a.info.ActorRef}, sender: remote.ActorRef}
	before := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
	now := time.Now()
	sequence := a.reachability.begin(remote.ActorRef, a.view.Members().List(), now, time.Second, 1)
	pong := gossipmessages.NewPong(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, remote.Clone(), before.MemberList, before.Version)
	pong.ProbeSequence, pong.ProbeOriginID = sequence, a.info.ID()
	a.onPong(ctx, pong)
	assert.Equal(t, vivid.ClusterReachabilityReachable, a.reachability.state(remote.ID()))
	assert.NotZero(t, a.viewRevision)
	assert.Same(t, before.MemberList, a.preparePing(gossipmessages.GossipTypeCycleSYNC).MemberList)
	a.refreshReachability(ctx, now.Add(a.opts.ReachabilityTTL+time.Second))
	assert.Equal(t, vivid.ClusterReachabilityUnknown, a.reachability.state(remote.ID()))
	assert.Same(t, before.MemberList, a.preparePong(gossipmessages.GossipTypeCycleSYNC).MemberList, "本地观察变化和过期不改变发送成员视图")
}

func TestActor_GossipOutboundViewRestart(t *testing.T) {
	a := New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterDataCenter("dc-a")))
	ctx := &viewStablePrelaunchContext{seedContext{ref: &tombstoneRef{address: "local:10001"}}}
	require.NoError(t, a.OnPrelaunch(ctx))
	before := a.preparePing(gossipmessages.GossipTypeJoin)
	previousID := before.Info.ID()
	a.opts.DataCenter = "dc-b"
	require.NoError(t, a.OnPrelaunch(ctx))
	after := a.preparePong(gossipmessages.GossipTypeJoin)
	assert.NotEqual(t, previousID, after.Info.ID())
	assert.NotSame(t, before.MemberList, after.MemberList)
	assert.Nil(t, after.MemberList.Get(previousID))
	assert.Equal(t, "dc-b", after.Info.DataCenter)
	assert.Equal(t, previousID, before.Info.ID())
	assert.Equal(t, "dc-a", before.Info.DataCenter)
}
