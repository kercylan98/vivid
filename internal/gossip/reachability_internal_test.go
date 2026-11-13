package gossip

import (
	"fmt"
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

func reachableMember(address string) *endpoint.Information {
	member := endpoint.NewInformation(&tombstoneRef{address: address}, time.Unix(1, 0))
	member.Status = endpoint.StatusUp
	return member
}

func TestLocalReachability_Attempts(t *testing.T) {
	now := time.Now()
	timeout, ttl := time.Second, 5*time.Second
	r := newLocalReachability()
	old := reachableMember("peer:10001")
	current := reachableMember("peer:10001")
	other := reachableMember("other:10002")
	members := []*endpoint.Information{old, current, other}
	assert.Equal(t, vivid.ClusterReachabilityUnknown, r.state(old.ID()))
	sequence := r.begin(old.ActorRef, members, now, timeout, 1)
	require.NotZero(t, sequence)
	assert.Equal(t, sequence, r.begin(old.ActorRef, members, now.Add(timeout/2), timeout, 1))
	assert.Zero(t, r.begin(other.ActorRef, members, now, timeout, 1), "等待数不超过目标额度")
	assert.Equal(t, now.Add(timeout), r.pending[old.ActorRef.GetAddress()].deadline, "重发不能延长预算")
	r.accept(other.ActorRef, sequence, other.ID(), now, ttl)
	r.accept(old.ActorRef, sequence+1, old.ID(), now, ttl)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, r.state(old.ID()))
	r.accept(current.ActorRef, sequence, current.ID(), now.Add(timeout/2), ttl)
	assert.Empty(t, r.pending)
	assert.Equal(t, vivid.ClusterReachabilityReachable, r.state(current.ID()))
	assert.Equal(t, vivid.ClusterReachabilityUnreachable, r.state(old.ID()), "同地址新实例的应答不能证明旧实例可达")
	r.changed = false
	r.accept(old.ActorRef, sequence, old.ID(), now.Add(timeout*3/4), ttl)
	assert.False(t, r.changed, "重复和迟到的旧响应不能续期")
	next := r.begin(current.ActorRef, members, now.Add(timeout), timeout, 1)
	require.NotEqual(t, sequence, next)
	r.expire(now.Add(timeout*2), ttl)
	assert.Empty(t, r.pending)
	assert.Equal(t, vivid.ClusterReachabilityUnreachable, r.state(current.ID()))
	r.accept(current.ActorRef, next, current.ID(), now.Add(timeout*2), ttl)
	assert.Equal(t, vivid.ClusterReachabilityUnreachable, r.state(current.ID()), "截止时间上的应答已过期")
	r.expire(now.Add(timeout*2+ttl), ttl)
	assert.Empty(t, r.observations)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, r.state(current.ID()))
	assert.Equal(t, endpoint.StatusUp, old.Status, "本地超时不改变成员生命周期")
}

func TestLocalReachability_RefreshForgetAndReset(t *testing.T) {
	now := time.Now()
	r := newLocalReachability()
	member := reachableMember("peer:10001")
	unknownSeed := &tombstoneRef{address: "seed:10002"}
	sequence := r.begin(unknownSeed, nil, now, time.Second, 2)
	r.accept(unknownSeed, sequence, "discovered-instance", now, time.Second)
	assert.Equal(t, vivid.ClusterReachabilityReachable, r.state("discovered-instance"))
	r.changed = false
	sequence = r.begin(unknownSeed, nil, now.Add(500*time.Millisecond), time.Second, 2)
	r.accept(unknownSeed, sequence, "discovered-instance", now.Add(500*time.Millisecond), time.Second)
	assert.False(t, r.changed, "连续成功只续期，不重复发布状态变化")
	r.expire(now.Add(time.Second), time.Second)
	assert.Equal(t, vivid.ClusterReachabilityReachable, r.state("discovered-instance"))
	r.expire(now.Add(1500*time.Millisecond), time.Second)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, r.state("discovered-instance"))
	r.begin(member.ActorRef, []*endpoint.Information{member}, now, time.Second, 2)
	r.forget(member.ID())
	r.expire(now.Add(time.Second), time.Second)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, r.state(member.ID()), "退出实例不能被未完成请求重新加入观察")
	r.observe(member.ID(), vivid.ClusterReachabilityReachable, now, time.Second)
	sequence = r.begin(member.ActorRef, []*endpoint.Information{member}, now, time.Second, 2)
	r.reset()
	assert.Empty(t, r.pending)
	assert.Empty(t, r.observations)
	r.accept(member.ActorRef, sequence, member.ID(), now, time.Second)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, r.state(member.ID()))
	// Actor 长时间未被调度时，不把很早就超时的请求当作刚刚失败来续期。
	r.changed = false
	r.begin(member.ActorRef, []*endpoint.Information{member}, now, time.Second, 2)
	r.expire(now.Add(10*time.Second), time.Second)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, r.state(member.ID()))
	assert.False(t, r.changed, "没有可见变化时不发布过期的中间结果")
}

type reachabilityContext struct {
	*tombstoneContext
	sender vivid.ActorRef
	reply  vivid.Message
	sent   []*gossipmessages.Ping
}

func (c *reachabilityContext) Sender() vivid.ActorRef      { return c.sender }
func (c *reachabilityContext) Reply(message vivid.Message) { c.reply = message }
func (c *reachabilityContext) Tell(_ vivid.ActorRef, message vivid.Message) {
	if ping, ok := message.(*gossipmessages.Ping); ok {
		c.sent = append(c.sent, ping)
	}
}

func TestActor_GossipReachabilityCorrelation(t *testing.T) {
	local, remote, relay := reachableMember("local:10001"), reachableMember("peer:10002"), reachableMember("relay:10003")
	a := &Actor{info: local, view: NewClusterView(log.GetDefault()), opts: vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")), reachability: newLocalReachability()}
	for _, member := range []*endpoint.Information{local, remote, relay} {
		a.view.Members().Upsert(member)
		a.view.Version().Update(member.ID(), 2)
	}
	ctx := &reachabilityContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}, sender: remote.ActorRef}
	initial := a.currentSnapshot()
	now := time.Now()
	sequence := a.reachability.begin(remote.ActorRef, a.view.Members().List(), now, time.Second, 1)
	pong := gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, remote, a.view.Members().Clone(), a.view.Version().Clone())
	pong.ProbeSequence, pong.ProbeOriginID = sequence, local.ID()
	ctx.sender = relay.ActorRef
	a.onPong(ctx, pong)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, a.reachability.state(remote.ID()), "转发消息不能刷新直接可达性")
	ctx.sender = remote.ActorRef
	pong.ProbeOriginID = "previous-local-instance"
	a.onPong(ctx, pong)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, a.reachability.state(remote.ID()))
	pong.ProbeOriginID = local.ID()
	a.onPong(ctx, pong)
	require.Len(t, ctx.stream.events, 1)
	event := ctx.stream.events[0].(ves.ClusterReachabilityChangedEvent)
	member, _ := event.Snapshot.Member(remote.ID())
	assert.Equal(t, vivid.ClusterReachabilityReachable, member.Reachability)
	assert.Equal(t, endpoint.StatusUp, member.Status)
	assert.Equal(t, uint64(2), member.Revision, "可达性变化不递增成员自身版本")
	before, _ := initial.Member(remote.ID())
	assert.Equal(t, vivid.ClusterReachabilityUnknown, before.Reachability, "已发布快照不随观察变化")
	assert.Same(t, event.Snapshot, a.currentSnapshot())
	a.onPong(ctx, pong)
	assert.Len(t, ctx.stream.events, 1)
	a.refreshReachability(ctx, now.Add(a.opts.ReachabilityTTL+time.Second))
	require.Len(t, ctx.stream.events, 2)
	expired := ctx.stream.events[1].(ves.ClusterReachabilityChangedEvent).Snapshot
	assert.Greater(t, expired.Revision(), event.Snapshot.Revision())
	member, _ = expired.Member(remote.ID())
	assert.Equal(t, vivid.ClusterReachabilityUnknown, member.Reachability)
	member, _ = event.Snapshot.Member(remote.ID())
	assert.Equal(t, vivid.ClusterReachabilityReachable, member.Reachability)
	assert.Equal(t, a.view.Version().Get(remote.ID()), uint64(2))
	// 停止后的直接应答仍不能恢复观察。
	a.reachability.begin(remote.ActorRef, a.view.Members().List(), time.Now(), time.Second, 1)
	a.stopping = true
	pong.ProbeSequence = a.reachability.pending[remote.ActorRef.GetAddress()].sequence
	a.onPong(ctx, pong)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, a.reachability.state(remote.ID()))
}

func TestActor_GossipReachabilityBudget(t *testing.T) {
	local := reachableMember("local:10001")
	opts := vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterGossipPeersCount(2))
	a := &Actor{info: local, view: NewClusterView(log.GetDefault()), opts: opts, reachability: newLocalReachability()}
	a.view.Members().Upsert(local)
	for i := range 10 {
		a.view.Members().Upsert(reachableMember(fmt.Sprintf("peer:%d", 10002+i)))
	}
	ctx := &reachabilityContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}}
	for range 20 {
		ctx.sent = nil
		a.onSpreadGossip(ctx, gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC))
		assert.Len(t, ctx.sent, 2, "观察额度耗尽也不能阻止原有视图交换")
		assert.LessOrEqual(t, len(a.reachability.pending), 2)
		for _, ping := range ctx.sent {
			assert.Equal(t, 11, len(ping.MemberList.List()))
		}
	}
	// Ping 的关联号应由应答原样返回，远端不使用请求方时间计算预算。
	remote := reachableMember("remote:20001")
	members, versions := memberlist.New(log.GetDefault()), versionvector.New()
	members.Upsert(remote)
	versions.Update(remote.ID(), 2)
	ping := gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, remote, members, versions)
	ping.ProbeSequence = 77
	a.onPing(ctx, ping)
	pong := ctx.reply.(*gossipmessages.Pong)
	assert.Equal(t, uint64(77), pong.ProbeSequence)
	assert.Equal(t, remote.ID(), pong.ProbeOriginID)
}

func TestActor_GossipReachabilityRequiresRoundTrip(t *testing.T) {
	local, remote := reachableMember("local:10001"), reachableMember("remote:10002")
	a := &Actor{info: local, view: NewClusterView(log.GetDefault()), opts: vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")), reachability: newLocalReachability()}
	a.view.Members().Upsert(local)
	a.view.Members().Upsert(remote)
	a.view.Version().Update(remote.ID(), 2)
	ctx := &reachabilityContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}, sender: remote.ActorRef}
	now := time.Now()
	sequence := a.reachability.begin(remote.ActorRef, a.view.Members().List(), now, time.Second, 1)
	ping := gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, remote, a.view.Members().Clone(), a.view.Version().Clone())
	a.onPing(ctx, ping)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, a.reachability.state(remote.ID()), "收到对方的单向请求不证明自己发出的请求得到应答")
	a.refreshReachability(ctx, now.Add(time.Second))
	assert.Equal(t, vivid.ClusterReachabilityUnreachable, a.reachability.state(remote.ID()))
	pong := gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, remote, ping.MemberList, ping.Version)
	pong.ProbeSequence, pong.ProbeOriginID = sequence, local.ID()
	a.onPong(ctx, pong)
	assert.Equal(t, vivid.ClusterReachabilityUnreachable, a.reachability.state(remote.ID()), "已完成超时的请求不能被迟到应答恢复")
	// 只有后续独立尝试的直接应答才能恢复。
	pong.ProbeSequence = a.reachability.begin(remote.ActorRef, a.view.Members().List(), time.Now(), time.Second, 1)
	a.onPong(ctx, pong)
	assert.Equal(t, vivid.ClusterReachabilityReachable, a.reachability.state(remote.ID()))
	// 第三方回复中转发的远端 Up 只更新成员视图，不刷新该远端的观察。
	a.reachability.observe(remote.ID(), vivid.ClusterReachabilityUnreachable, time.Now(), time.Second)
	relay := reachableMember("relay:10003")
	ping.MemberList.Upsert(relay)
	ping.Version.Update(relay.ID(), 2)
	pong = gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, relay, ping.MemberList, ping.Version)
	ctx.sender = relay.ActorRef
	pong.ProbeSequence = a.reachability.begin(relay.ActorRef, a.view.Members().List(), time.Now(), time.Second, 1)
	pong.ProbeOriginID = local.ID()
	a.onPong(ctx, pong)
	assert.Equal(t, vivid.ClusterReachabilityReachable, a.reachability.state(relay.ID()))
	assert.Equal(t, vivid.ClusterReachabilityUnreachable, a.reachability.state(remote.ID()))
}

func TestActor_GossipReachabilityRemoved(t *testing.T) {
	local, remote := reachableMember("local:10001"), reachableMember("remote:10002")
	a := &Actor{info: local, view: NewClusterView(log.GetDefault()), opts: vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")), reachability: newLocalReachability()}
	a.view.Members().Upsert(local)
	a.view.Members().Upsert(remote)
	a.view.Version().Update(remote.ID(), 2)
	ctx := &reachabilityContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}, sender: remote.ActorRef}
	a.reachability.observe(remote.ID(), vivid.ClusterReachabilityReachable, time.Now(), time.Hour)
	a.reachability.changed = false
	before := a.currentSnapshot()
	sequence := a.reachability.begin(remote.ActorRef, a.view.Members().List(), time.Now(), time.Second, 1)
	removed := remote.Clone()
	removed.Status = endpoint.StatusRemoved
	members, versions := a.view.Members().Clone(), a.view.Version().Clone()
	members.Upsert(removed)
	versions.Update(remote.ID(), 5)
	pong := gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeRemoveNode, removed, members, versions)
	pong.ProbeSequence, pong.ProbeOriginID = sequence, local.ID()
	a.onPong(ctx, pong)
	member, _ := a.currentSnapshot().Member(remote.ID())
	assert.Equal(t, vivid.ClusterReachabilityUnknown, member.Reachability)
	assert.Equal(t, endpoint.StatusRemoved, member.Status)
	require.Len(t, ctx.stream.events, 2, "成员与可达性订阅者都应获知退出")
	assert.IsType(t, ves.ClusterMembersChangedEvent{}, ctx.stream.events[0])
	assert.IsType(t, ves.ClusterReachabilityChangedEvent{}, ctx.stream.events[1])
	member, _ = before.Member(remote.ID())
	assert.Equal(t, vivid.ClusterReachabilityReachable, member.Reachability)
	// 旧 Up 应答和未完成请求超时均不能恢复已退出实例的观察。
	pong.Info = remote
	a.onPong(ctx, pong)
	a.refreshReachability(ctx, time.Now().Add(time.Second))
	assert.Equal(t, vivid.ClusterReachabilityUnknown, a.reachability.state(remote.ID()))
	assert.Equal(t, endpoint.StatusRemoved, a.view.Members().Get(remote.ID()).Status)
}

type reachabilitySpreadContext struct {
	*tombstoneContext
	tracker *localReachability
	ids     map[string]string
}

func (c *reachabilitySpreadContext) Tell(peer vivid.ActorRef, message vivid.Message) {
	if c.tracker != nil {
		ping := message.(*gossipmessages.Ping)
		c.tracker.accept(peer, ping.ProbeSequence, c.ids[peer.GetAddress()], time.Now(), time.Hour)
	}
}

// BenchmarkActor_GossipReachability 比较相同视图发送流程是否记录直接应答的本地成本，不包含网络与远端合并。
func BenchmarkActor_GossipReachability(b *testing.B) {
	for _, count := range []int{10, 100, 1000} {
		for _, observe := range []bool{false, true} {
			b.Run(fmt.Sprintf("members_%d/observe_%t", count, observe), func(b *testing.B) {
				local := reachableMember("local:10001")
				a := &Actor{info: local, view: NewClusterView(log.GetDefault()), opts: vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"))}
				ctx := &reachabilitySpreadContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}, ids: make(map[string]string)}
				if observe {
					a.reachability = newLocalReachability()
					ctx.tracker = a.reachability
				}
				a.view.Members().Upsert(local)
				for i := 1; i < count; i++ {
					member := reachableMember(fmt.Sprintf("peer:%d", 10001+i))
					a.view.Members().Upsert(member)
					ctx.ids[member.ActorRef.GetAddress()] = member.ID()
				}
				message := gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC)
				// 预热观察，计时只包含稳定结果的续期，不累积测试事件。
				for range count * 10 {
					a.onSpreadGossip(ctx, message)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					a.onSpreadGossip(ctx, message)
				}
			})
		}
	}
}
