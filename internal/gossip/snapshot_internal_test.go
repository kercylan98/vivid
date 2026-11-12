package gossip

import (
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

func TestActor_GossipSnapshotChanges(t *testing.T) {
	local := endpoint.NewInformation(&tombstoneRef{address: "local:10001"}, time.Unix(1, 0))
	local.Status = endpoint.StatusUp
	a := &Actor{info: local, view: NewClusterView(log.GetDefault())}
	a.view.Members().Upsert(local)
	a.view.Version().Update(local.ID(), 2)
	ctx := &tombstoneContext{ref: local.ActorRef}
	initial := a.currentSnapshot()
	assert.Equal(t, local.ID(), initial.LocalID())
	assert.Equal(t, 1, initial.Len())
	_, found := initial.Member("unknown")
	assert.False(t, found)

	remote := endpoint.NewInformation(&tombstoneRef{address: "remote:10002"}, time.Unix(2, 0))
	remote.Status = endpoint.StatusUp
	old := endpoint.NewInformation(remote.ActorRef.Clone(), time.Unix(1, 0))
	old.Status = endpoint.StatusRemoved
	members := memberlist.New(log.GetDefault())
	members.Upsert(remote)
	members.Upsert(old)
	version := versionvector.New()
	version.Update(remote.ID(), 2)
	version.Update(old.ID(), 5)
	ping := gossipmessages.NewPing(gossipmessages.GossipTypeCycleSYNC, remote, members, version)
	a.onPing(ctx, ping)
	require.Len(t, ctx.stream.events, 1, "一批成员更新只发布一次完整视图")
	event := ctx.stream.events[0].(ves.ClusterMembersChangedEvent)
	assert.Equal(t, uint64(1), event.Snapshot.Revision())
	assert.Equal(t, 3, event.Snapshot.Len())
	assert.Same(t, event.Snapshot, a.currentSnapshot(), "未变更时查询复用只读快照")
	assert.Equal(t, 1, initial.Len(), "后续成员发现不能改变旧快照")
	member, found := event.Snapshot.Member(old.ID())
	require.True(t, found)
	assert.Equal(t, vivid.ClusterMemberStatusRemoved, member.Status)
	assert.Equal(t, uint64(5), member.Revision)
	assert.Equal(t, remote.ActorRef.GetAddress(), member.Address)

	// 订阅者得到的切片和值都可独立修改，不影响其他订阅者或后续查询。
	copy := event.Snapshot.Members()
	copy[0].ID = "changed"
	copy[0].Address = "changed"
	copy[0].Status = vivid.ClusterMemberStatusNone
	assert.NotEqual(t, copy, event.Snapshot.Members())
	member.Status = vivid.ClusterMemberStatusNone
	unchanged, _ := event.Snapshot.Member(old.ID())
	assert.Equal(t, vivid.ClusterMemberStatusRemoved, unchanged.Status)

	for range 3 {
		a.onPing(ctx, ping)
		a.onPong(ctx, gossipmessages.NewPong(ping.GossipType, remote, members, version))
	}
	require.Len(t, ctx.stream.events, 1, "重复视图不得重复发布事件")
	stale := remote.Clone()
	stale.Status = endpoint.StatusJoining
	staleVersion := versionvector.New()
	staleVersion.Update(stale.ID(), 1)
	a.onPong(ctx, gossipmessages.NewPong(ping.GossipType, stale, members, staleVersion))
	require.Len(t, ctx.stream.events, 1, "旧版本不得触发事件")

	next := remote.Clone()
	next.Status = endpoint.StatusLeaving
	version = version.Clone()
	version.Update(next.ID(), 3)
	a.onPong(ctx, gossipmessages.NewPong(ping.GossipType, next, members, version))
	require.Len(t, ctx.stream.events, 2)
	latest := ctx.stream.events[1].(ves.ClusterMembersChangedEvent).Snapshot
	assert.Equal(t, uint64(2), latest.Revision())
	before, _ := event.Snapshot.Member(remote.ID())
	after, _ := latest.Member(remote.ID())
	assert.Equal(t, vivid.ClusterMemberStatusUp, before.Status)
	assert.Equal(t, vivid.ClusterMemberStatusLeaving, after.Status)
	assert.Equal(t, uint64(3), after.Revision)

	assert.True(t, changeStatus(ctx, a, EventLeaving))
	require.Len(t, ctx.stream.events, 3)
	localEvent := ctx.stream.events[2].(ves.ClusterMembersChangedEvent).Snapshot
	localMember, _ := localEvent.Member(local.ID())
	assert.Equal(t, vivid.ClusterMemberStatusLeaving, localMember.Status)
	assert.Equal(t, uint64(3), localMember.Revision)
	assert.Equal(t, uint64(3), localEvent.Revision())
	assert.False(t, changeStatus(ctx, a, EventUp))
	assert.Len(t, ctx.stream.events, 3, "非法状态迁移不得触发事件")
	assert.True(t, changeStatus(ctx, a, EventExiting))
	assert.True(t, changeStatus(ctx, a, EventRemoved))
	require.Len(t, ctx.stream.events, 5)
	removed := ctx.stream.events[4].(ves.ClusterMembersChangedEvent).Snapshot
	removedLocal, _ := removed.Member(local.ID())
	assert.Equal(t, vivid.ClusterMemberStatusRemoved, removedLocal.Status)
	assert.Equal(t, uint64(5), removedLocal.Revision)
	assert.Equal(t, vivid.ClusterMemberStatusLeaving, localMember.Status, "退出推进不能改变已发布快照")
}
