package gossip

import (
	"strings"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_GossipDataCenterDeclaration(t *testing.T) {
	a := New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterDataCenter("dc-a")))
	ctx := &viewStablePrelaunchContext{seedContext{ref: &tombstoneRef{address: "local:10001"}}}
	require.NoError(t, a.OnPrelaunch(ctx))
	initial := a.info.Clone()
	assert.Equal(t, "dc-a", initial.DataCenter)
	assert.Equal(t, "dc-a", a.currentSnapshot().Members()[0].DataCenter)
	a.opts.DataCenter = "dc-b"
	assert.Equal(t, "dc-a", a.preparePing(gossipmessages.GossipTypeCycleSYNC).Info.DataCenter, "启动声明不随选项修改而变化")
	assert.Equal(t, "dc-a", a.preparePong(gossipmessages.GossipTypeCycleSYNC).Info.DataCenter)
	require.NoError(t, a.OnPrelaunch(ctx))
	assert.NotEqual(t, initial.ID(), a.info.ID())
	assert.Equal(t, "dc-b", a.info.DataCenter, "重新启动的实例可以重新声明机房")
	assert.Equal(t, "dc-a", initial.DataCenter, "克隆保留原实例声明")
}

func TestActor_GossipDataCenterBoundary(t *testing.T) {
	for _, reply := range []bool{false, true} {
		for _, direct := range []bool{false, true} {
			name := "ping/relayed"
			if reply {
				name = "pong/relayed"
			}
			if direct {
				name += "/direct"
			}
			t.Run(name, func(t *testing.T) {
				for _, label := range []string{"", "dc-b", "dc a", "dc\n", string([]byte{0xff}), strings.Repeat("x", 129)} {
					a, _ := newViewStableActor(endpoint.StatusUp)
					a.info.DataCenter = "dc-local"
					remote := reachableMember("remote:10002")
					remote.DataCenter = "dc-a"
					a.view.Members().Upsert(remote)
					a.view.Version().Update(remote.ID(), 1)
					changed := remote.Clone()
					changed.DataCenter = label
					members := memberlist.New(log.GetDefault())
					members.Upsert(changed)
					version := versionvector.New()
					version.Update(changed.ID(), 2)
					sender := changed
					if !direct {
						sender = reachableMember("relay:10003")
						sender.DataCenter = "dc-relay"
						a.view.Members().Upsert(sender)
						a.view.Version().Update(sender.ID(), 1)
						members.Upsert(sender)
						version.Update(sender.ID(), 1)
					}
					before := a.currentSnapshot()
					ctx := &tombstoneContext{ref: a.info.ActorRef}
					if reply {
						a.onPong(ctx, gossipmessages.NewPong(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, sender, members, version))
					} else {
						a.onPing(ctx, gossipmessages.NewPing(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, sender, members, version))
					}
					assert.Equal(t, "dc-a", a.view.Members().Get(remote.ID()).DataCenter)
					assert.Equal(t, uint64(1), a.view.Version().Get(remote.ID()), "机房冲突不能接收其新版本")
					assert.Same(t, before, a.currentSnapshot())
					assert.Nil(t, ctx.stream, "无有效变更时不发布事件")
				}
			})
		}
	}
}

func TestActor_GossipDataCenterInvalidNewMember(t *testing.T) {
	for _, label := range []string{"dc a", "dc\n", string([]byte{0xff}), strings.Repeat("x", 129)} {
		a, ctx := newViewStableActor(endpoint.StatusUp)
		relay, invalid := reachableMember("relay:10002"), reachableMember("invalid:10003")
		relay.DataCenter, invalid.DataCenter = "dc-relay", label
		members := memberlist.New(log.GetDefault())
		members.Upsert(invalid)
		version := versionvector.New()
		version.Update(relay.ID(), 1)
		version.Update(invalid.ID(), 1)
		a.onPong(ctx, gossipmessages.NewPong(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, invalid, members, version))
		require.Len(t, a.view.Members().List(), 1, "标识无效的直接应答不能进入视图")
		a.onPong(ctx, gossipmessages.NewPong(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, relay, members, version))
		require.Len(t, a.view.Members().List(), 2, "有效转发者可以进入视图")
		assert.Nil(t, a.view.Members().Get(invalid.ID()))
		assert.Zero(t, a.view.Version().Get(invalid.ID()), "不导入被拒绝记录的版本")
	}
}

func TestActor_GossipDataCenterHistory(t *testing.T) {
	a, ctx := newViewStableActor(endpoint.StatusUp)
	a.info.DataCenter = "dc-local"
	old := reachableMember("remote:10002")
	old.DataCenter = "dc-a"
	a.view.Members().Upsert(old)
	a.view.Version().Update(old.ID(), 1)
	before := a.currentSnapshot()
	next := endpoint.NewInformation(old.ActorRef.Clone(), time.Unix(2, 0))
	next.Status, next.DataCenter = endpoint.StatusUp, "dc-b"
	members := memberlist.New(log.GetDefault())
	members.Upsert(next)
	version := versionvector.New()
	version.Update(next.ID(), 1)
	a.onPong(ctx, gossipmessages.NewPong(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, next, members, version))
	require.Len(t, ctx.stream.events, 1)
	snapshot := ctx.stream.events[0].(ves.ClusterMembersChangedEvent).Snapshot
	previous, found := snapshot.Member(old.ID())
	require.True(t, found)
	assert.Equal(t, "dc-a", previous.DataCenter)
	current, found := snapshot.Member(next.ID())
	require.True(t, found)
	assert.Equal(t, "dc-b", current.DataCenter)
	assert.Equal(t, previous.Address, current.Address)
	assert.Equal(t, 2, before.Len())
	assert.Equal(t, 3, snapshot.Len())
	copy := snapshot.Members()
	for index := range copy {
		copy[index].DataCenter = "modified"
	}
	current.DataCenter = "modified"
	current, _ = snapshot.Member(next.ID())
	assert.Equal(t, "dc-b", current.DataCenter, "修改成员值与返回切片不影响其他订阅者")
	previous, _ = before.Member(old.ID())
	assert.Equal(t, "dc-a", previous.DataCenter)
	a.reachability = newLocalReachability()
	a.reachability.observe(next.ID(), vivid.ClusterReachabilityReachable, time.Now(), time.Second)
	a.publishReachabilityChanged(ctx)
	require.Len(t, ctx.stream.events, 2)
	observed := ctx.stream.events[1].(ves.ClusterReachabilityChangedEvent).Snapshot
	current, found = observed.Member(next.ID())
	require.True(t, found)
	assert.Equal(t, "dc-b", current.DataCenter, "可达性事件保留实例机房声明")
	assert.Equal(t, vivid.ClusterReachabilityReachable, current.Reachability)
	current, _ = snapshot.Member(next.ID())
	assert.Equal(t, vivid.ClusterReachabilityUnknown, current.Reachability, "后续观察不改变成员事件的历史快照")
}
