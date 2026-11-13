package gossip

import (
	"slices"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPongAckActors() (*Actor, *Actor, *reachabilityContext, *reachabilityContext) {
	a, _ := newViewStableActor(endpoint.StatusUp)
	b, _ := newViewStableActor(endpoint.StatusUp)
	b.info.ActorRef = &tombstoneRef{address: "remote:10002"}
	a.info.DataCenter, b.info.DataCenter = "dc-a", "dc-b"
	a.reachability, b.reachability = newLocalReachability(), newLocalReachability()
	a.mergeMember(b.info.Clone(), b.view.Version())
	b.mergeMember(a.info.Clone(), a.view.Version())
	a.membersChanged, b.membersChanged = false, false
	aCtx := &reachabilityContext{tombstoneContext: &tombstoneContext{ref: a.info.ActorRef}, sender: b.info.ActorRef}
	bCtx := &reachabilityContext{tombstoneContext: &tombstoneContext{ref: b.info.ActorRef}, sender: a.info.ActorRef}
	return a, b, aCtx, bCtx
}

func TestActor_GossipViewMatch(t *testing.T) {
	a, b, _, _ := newPongAckActors()
	initial := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
	require.True(t, b.currentOutboundView().matches(initial.MemberList, initial.Version), "不同节点可识别同一完整视图")
	for _, field := range []string{"id", "data_center", "address", "path", "virtual", "status", "created", "version", "orphan", "removed"} {
		t.Run(field, func(t *testing.T) {
			view := &outboundView{members: a.view.Members().Clone(), version: a.view.Version().Clone()}
			member := view.members.List()[0]
			switch field {
			case "id":
				member.InstanceID = "different"
			case "data_center":
				member.DataCenter = "dc-c"
			case "address":
				member.ActorRef = &tombstoneRef{address: "different:20001"}
			case "path":
				member.ActorRef = &viewPathRef{tombstoneRef: &tombstoneRef{address: member.ActorRef.GetAddress()}}
			case "virtual":
				member.ActorRef = &viewVirtualRef{tombstoneRef: &tombstoneRef{address: member.ActorRef.GetAddress()}}
			case "status":
				member.Status = endpoint.StatusRemoved
			case "removed":
				view.members = memberlist.New(nil)
			case "created":
				member.CreatedAt = member.CreatedAt.Add(time.Nanosecond)
			case "version":
				view.version.Increment(member.ID())
			case "orphan":
				view.version.Increment("unknown-version")
			}
			assert.False(t, b.currentOutboundView().matches(view.members, view.version))
		})
	}
	view := &outboundView{members: a.view.Members().Clone(), version: a.view.Version().Clone()}
	slices.Reverse(view.members.List())
	assert.True(t, b.currentOutboundView().matches(view.members, view.version), "成员顺序不影响比较")
	for _, field := range []string{"duplicate", "nil_info", "nil_ref"} {
		t.Run(field, func(t *testing.T) {
			members := initial.MemberList.Clone()
			switch field {
			case "duplicate":
				members.List()[1] = members.List()[0]
			case "nil_info":
				members.List()[0] = nil
			case "nil_ref":
				members.List()[0].ActorRef = nil
			}
			assert.False(t, b.currentOutboundView().matches(members, initial.Version))
			assert.True(t, b.currentOutboundView().matches(initial.MemberList, initial.Version), "失败后仍可比较下一份视图")
		})
	}
	b.currentOutboundView().round = ^uint64(0)
	assert.True(t, b.currentOutboundView().matches(initial.MemberList, initial.Version), "计数回绕后清空去重标记")
	changed := b.info.Clone()
	changed.Status = endpoint.StatusRemoved
	version := b.view.Version().Clone()
	version.Increment(changed.ID())
	a.mergeMember(changed, version)
	assert.False(t, a.currentOutboundView().matches(initial.MemberList, initial.Version))
	assert.Equal(t, endpoint.StatusUp, initial.MemberList.Get(changed.ID()).Status, "缓存失效不能修改旧消息")
}

type viewPathRef struct{ *tombstoneRef }

func (*viewPathRef) GetPath() string { return "/different" }

type viewVirtualRef struct{ *tombstoneRef }

func (*viewVirtualRef) IsVirtual() bool { return true }

func TestActor_GossipPongAckFallback(t *testing.T) {
	for _, name := range []string{"equal", "missing", "different", "duplicate", "join", "newer_receiver", "merged_sender"} {
		t.Run(name, func(t *testing.T) {
			a, b, _, ctx := newPongAckActors()
			if name == "merged_sender" {
				remote := reachableMember("third:10003")
				version := a.view.Version().Clone()
				version.Update(remote.ID(), 1)
				a.mergeMember(remote, version)
			}
			ping := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
			ping.ProbeSequence = 17
			switch name {
			case "missing":
				ping.MemberList = memberlist.New(nil)
				ping.MemberList.Upsert(a.info.Clone())
			case "different":
				ping.Version = ping.Version.Clone()
				ping.Version.Increment("unknown-version")
			case "duplicate":
				ping.MemberList = ping.MemberList.Clone()
				ping.MemberList.List()[1] = ping.MemberList.List()[0]
			case "join":
				ping.GossipType = gossipmessages.GossipTypeJoin
			case "newer_receiver":
				version := b.view.Version().Clone()
				version.Increment(a.info.ID())
				b.mergeMember(a.info.Clone(), version)
			}
			b.onPing(ctx, ping)
			if name == "equal" || name == "merged_sender" {
				ack, ok := ctx.reply.(*gossipmessages.PongAck)
				require.True(t, ok)
				assert.Equal(t, uint64(17), ack.ProbeSequence)
				assert.Equal(t, a.info.ID(), ack.ProbeOriginID)
				assert.Equal(t, b.info.ID(), ack.Info.ID())
				assert.Equal(t, b.view.Version().Get(b.info.ID()), ack.Revision)
			} else {
				pong, ok := ctx.reply.(*gossipmessages.Pong)
				require.True(t, ok, "视图不完整、不一致或加入时必须返回完整视图")
				assert.NotNil(t, pong.MemberList)
				assert.NotNil(t, pong.Version)
				assert.Equal(t, uint64(17), pong.ProbeSequence)
			}
		})
	}
}

func TestActor_GossipPongAckCorrelation(t *testing.T) {
	for _, name := range []string{"valid", "no_sequence", "old_sequence", "old_origin", "wrong_sender", "no_sender", "unknown_instance", "wrong_address", "old_revision", "new_revision", "zero_revision", "data_center", "status", "created", "expired", "stopping", "joining", "wrong_cluster", "nil_info", "nil_ref"} {
		t.Run(name, func(t *testing.T) {
			a, b, ctx, replyCtx := newPongAckActors()
			now := time.Now()
			ping := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
			ping.ProbeSequence = a.reachability.begin(b.info.ActorRef, a.view.Members().List(), now, time.Second, 1)
			b.onPing(replyCtx, ping)
			original, ok := replyCtx.reply.(*gossipmessages.PongAck)
			require.True(t, ok)
			ack := *original
			ack.Info = ack.Info.Clone()
			switch name {
			case "no_sequence":
				ack.ProbeSequence = 0
			case "old_sequence":
				ack.ProbeSequence++
			case "old_origin":
				ack.ProbeOriginID = "old-local-instance"
			case "wrong_sender":
				ctx.sender = a.info.ActorRef
			case "no_sender":
				ctx.sender = nil
			case "unknown_instance":
				ack.Info.InstanceID = "new-instance-at-same-address"
			case "wrong_address":
				ack.Info.ActorRef = &tombstoneRef{address: "wrong:20001"}
				ctx.sender = ack.Info.ActorRef
			case "old_revision":
				a.view.Version().Increment(b.info.ID())
			case "zero_revision":
				ack.Revision = 0
			case "new_revision":
				ack.Revision++
			case "data_center":
				ack.Info.DataCenter = "dc-c"
			case "status":
				ack.Info.Status = endpoint.StatusLeaving
			case "created":
				ack.Info.CreatedAt = ack.Info.CreatedAt.Add(time.Second)
			case "expired":
				probe := a.reachability.pending[b.info.ActorRef.GetAddress()]
				probe.deadline = now.Add(-time.Second)
				a.reachability.pending[b.info.ActorRef.GetAddress()] = probe
			case "stopping":
				a.stopping = true
			case "joining":
				a.info.Status = endpoint.StatusJoining
			case "wrong_cluster":
				ack.ClusterName = "different"
			case "nil_info":
				ack.Info = nil
			case "nil_ref":
				ack.Info.ActorRef = nil
			}
			fingerprint := a.view.Fingerprint()
			a.onPongAck(ctx, &ack)
			assert.Equal(t, fingerprint, a.view.Fingerprint(), "轻量应答不能更新成员或版本")
			if name == "valid" {
				assert.Equal(t, vivid.ClusterReachabilityReachable, a.reachability.state(b.info.ID()))
				assert.Empty(t, a.reachability.pending)
				a.onPongAck(ctx, &ack)
				assert.Len(t, ctx.stream.events, 1, "重复响应不重复确认")
			} else {
				assert.NotEqual(t, vivid.ClusterReachabilityReachable, a.reachability.state(b.info.ID()))
			}
		})
	}
}

func TestActor_GossipPongAckRecovery(t *testing.T) {
	a, b, ctx, replyCtx := newPongAckActors()
	ping := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
	ping.ProbeSequence = a.reachability.begin(b.info.ActorRef, a.view.Members().List(), time.Now(), time.Second, 1)
	b.onPing(replyCtx, ping)
	_, ok := replyCtx.reply.(*gossipmessages.PongAck)
	require.True(t, ok)
	// 丢失轻量响应后，仍可沿用请求重发完整 Ping，不依赖保存对端视图或额外重试状态。
	b.onPing(replyCtx, ping)
	a.onPongAck(ctx, replyCtx.reply.(*gossipmessages.PongAck))
	assert.Equal(t, vivid.ClusterReachabilityReachable, a.reachability.state(b.info.ID()))
	newMember := reachableMember("third:10003")
	version := b.view.Version().Clone()
	version.Update(newMember.ID(), 1)
	b.mergeMember(newMember, version)
	ping = a.preparePing(gossipmessages.GossipTypeCycleSYNC)
	b.onPing(replyCtx, ping)
	pong, ok := replyCtx.reply.(*gossipmessages.Pong)
	require.True(t, ok, "视图变化后恢复完整同步")
	a.onPong(ctx, pong)
	require.NotNil(t, a.view.Members().Get(newMember.ID()))
	ping = a.preparePing(gossipmessages.GossipTypeCycleSYNC)
	b.onPing(replyCtx, ping)
	_, ok = replyCtx.reply.(*gossipmessages.PongAck)
	assert.True(t, ok, "全量恢复完成后可重新轻量应答")
	assert.NotNil(t, pong.MemberList.Get(newMember.ID()))
}
