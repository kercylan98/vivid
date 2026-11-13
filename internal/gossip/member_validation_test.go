package gossip

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func invalidMember(info *endpoint.Information, field string) *endpoint.Information {
	if field == "nil" {
		return nil
	}
	info = info.Clone()
	switch field {
	case "nil_ref":
		info.ActorRef = nil
	case "empty_id":
		info.InstanceID = ""
	case "none":
		info.Status = endpoint.StatusNone
	case "unknown_status":
		info.Status = endpoint.Status(255)
	case "address":
		info.ActorRef = &tombstoneRef{address: "wrong:20001"}
	case "invalid_address":
		info.ActorRef = &tombstoneRef{address: "bad host:0"}
	case "padded_address":
		info.ActorRef = &tombstoneRef{address: " remote:10002 "}
	case "path":
		info.ActorRef = &viewPathRef{tombstoneRef: &tombstoneRef{address: info.ActorRef.GetAddress()}}
	case "virtual":
		info.ActorRef = &viewVirtualRef{tombstoneRef: &tombstoneRef{address: info.ActorRef.GetAddress()}}
	case "created":
		info.CreatedAt = info.CreatedAt.Add(time.Nanosecond)
	case "data_center":
		info.DataCenter = "dc-c"
	}
	return info
}

func TestActor_GossipMemberValidation(t *testing.T) {
	for _, protocol := range []string{"ping", "pong"} {
		for _, direct := range []bool{false, true} {
			for _, field := range []string{"nil", "nil_ref", "empty_id", "none", "unknown_status", "address", "invalid_address", "padded_address", "path", "virtual", "created", "data_center", "zero_version"} {
				t.Run(protocol+"/"+map[bool]string{true: "direct", false: "relayed"}[direct]+"/"+field, func(t *testing.T) {
					a, b, ctx, _ := newPongAckActors()
					original := a.currentSnapshot()
					outbound := a.currentOutboundView()
					version := a.view.Version().Clone()
					bad := invalidMember(b.info, field)
					if field != "zero_version" {
						version.Increment(b.info.ID())
					} else {
						// 对同一实例的零版本仅应被忽略，不能覆盖已知版本。
						version = nil
					}
					good := reachableMember("third:10003")
					members := memberlist.New(nil)
					members.Upsert(good)
					members.Upsert(b.info.Clone())
					records := members.List()
					records[1] = bad
					members = memberRecords(records)
					if version == nil {
						version = b.view.Version().Clone()
						version.Remove(b.info.ID())
					}
					version.Update(good.ID(), 1)
					sender := a.info.Clone()
					if direct {
						sender = bad
					}
					if protocol == "ping" {
						a.onPing(ctx, gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, sender, members, version))
					} else {
						a.onPong(ctx, gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, sender, members, version))
					}
					assert.Equal(t, uint64(1), a.view.Version().Get(b.info.ID()))
					assert.Equal(t, b.info, a.view.Members().Get(b.info.ID()))
					if direct {
						assert.Same(t, original, a.currentSnapshot(), "无效发送方不能发布新快照")
						assert.Same(t, outbound, a.currentOutboundView())
						assert.Nil(t, ctx.reply)
						assert.Nil(t, a.view.Members().Get(good.ID()))
					} else {
						require.NotNil(t, a.view.Members().Get(good.ID()), "无效转发记录不阻断合法记录")
						assert.Equal(t, uint64(1), a.view.Version().Get(good.ID()))
						assert.Equal(t, 2, original.Len(), "旧快照保持不变")
					}
				})
			}
		}
	}
}

func TestActor_GossipMemberNewIdentity(t *testing.T) {
	for _, status := range []endpoint.Status{endpoint.StatusJoining, endpoint.StatusUp, endpoint.StatusLeaving, endpoint.StatusExiting, endpoint.StatusRemoved} {
		t.Run(status.String(), func(t *testing.T) {
			a, b, _, _ := newPongAckActors()
			restarted := endpoint.NewInformation(b.info.ActorRef.Clone(), b.info.CreatedAt.Add(time.Second))
			restarted.DataCenter, restarted.Status = "dc-c", status
			version := a.view.Version().Clone()
			version.Update(restarted.ID(), 1)
			require.True(t, a.mergeMember(restarted, version), "同地址新实例可声明不同机房和创建时间")
			assert.Equal(t, restarted, a.view.Members().Get(restarted.ID()))
			assert.Equal(t, b.info, a.view.Members().Get(b.info.ID()))
		})
	}
	for _, field := range []string{"nil", "nil_ref", "empty_id", "none", "unknown_status", "invalid_address", "padded_address", "path", "virtual"} {
		t.Run(field, func(t *testing.T) {
			a, _, _, _ := newPongAckActors()
			member := reachableMember("third:10003")
			version := a.view.Version().Clone()
			version.Update(member.ID(), 1)
			before := a.currentOutboundView()
			assert.False(t, a.mergeMember(invalidMember(member, field), version))
			assert.Nil(t, a.view.Members().Get(member.ID()))
			assert.Zero(t, a.view.Version().Get(member.ID()))
			assert.Same(t, before, a.currentOutboundView())
		})
	}
}

func TestActor_GossipMemberRejectedVersionRecovery(t *testing.T) {
	for _, protocol := range []string{"ping", "pong"} {
		t.Run(protocol, func(t *testing.T) {
			a, b, ctx, _ := newPongAckActors()
			original, outbound := a.currentSnapshot(), a.currentOutboundView()
			bad := b.info.Clone()
			bad.Status = endpoint.Status(255)
			version := b.view.Version().Clone()
			version.Update(bad.ID(), 100)
			members := memberlist.New(nil)
			members.Upsert(bad)
			send := func(sender *endpoint.Information) {
				if protocol == "ping" {
					a.onPing(ctx, gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, sender, members, version))
				} else {
					a.onPong(ctx, gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, sender, members, version))
				}
			}
			send(a.info.Clone())
			assert.Same(t, original, a.currentSnapshot())
			assert.Same(t, outbound, a.currentOutboundView())
			assert.Nil(t, ctx.stream, "单条被拒绝的转发不能发布成员事件")
			good := b.info.Clone()
			good.Status = endpoint.StatusLeaving
			members = memberlist.New(nil)
			members.Upsert(good)
			version = b.view.Version().Clone()
			version.Update(good.ID(), 2)
			send(good)
			assert.Equal(t, endpoint.StatusLeaving, a.view.Members().Get(good.ID()).Status)
			assert.Equal(t, uint64(2), a.view.Version().Get(good.ID()), "拒绝版本 100 后仍接受合法版本 2")
			member, exists := original.Member(good.ID())
			require.True(t, exists)
			assert.Equal(t, endpoint.StatusUp, member.Status)
		})
	}
}
