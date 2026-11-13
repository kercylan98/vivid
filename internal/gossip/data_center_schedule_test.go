package gossip

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataCenterSchedule_Coverage(t *testing.T) {
	for _, limit := range []int{1, 2, 5, 200} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			r := newLocalReachability()
			var members []*endpoint.Information
			var peers []vivid.ActorRef
			labels := make(map[string]string)
			for index := range 100 {
				member := reachableMember(fmt.Sprintf("peer:%d", 10001+index))
				member.DataCenter = "dc-a"
				if index >= 90 {
					member.DataCenter = "dc-b"
				}
				if index == 98 {
					member.DataCenter = "dc-c"
				}
				if index == 99 {
					member.DataCenter = ""
				}
				members, peers = append(members, member), append(peers, member.ActorRef)
				labels[member.ActorRef.GetAddress()] = member.DataCenter
			}
			random := rand.New(rand.NewPCG(1, 2))
			now := time.Now()
			started := make(map[string]bool)
			groups := make(map[string]bool)
			seen := make(map[string]map[string]bool)
			for round := range (4*90 + limit - 1) / limit {
				candidates := slices.Clone(peers)
				random.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
				selected := r.selectDataCenterPeers(candidates, members, limit)
				assert.ElementsMatch(t, peers, candidates, "重排不能重复或遗漏地址")
				require.Len(t, selected, min(limit, len(peers)))
				for _, peer := range selected {
					address := peer.GetAddress()
					label := labels[address]
					if limit <= 4 {
						require.False(t, groups[label], "小机房也应在机房轮换中获得新观察，不能被大机房地址数量淹没")
						groups[label] = true
						if len(groups) == 4 {
							clear(groups)
						}
					}
					if seen[label] == nil {
						seen[label] = make(map[string]bool)
					}
					if len(seen[label]) < map[string]int{"dc-a": 90, "dc-b": 8, "dc-c": 1, "": 1}[label] {
						require.False(t, seen[label][address], "机房内部也应轮换地址")
					}
					seen[label][address], started[address] = true, true
					require.NotZero(t, r.begin(peer, nil, now, time.Second, limit))
				}
				assert.LessOrEqual(t, len(r.pending), limit)
				// 无响应也按预算释放机会，不以机房中某个地址成功为轮换前提。
				now = now.Add(time.Second)
				r.expire(now, time.Second)
				if round == 0 {
					assert.Len(t, r.dataCenters.groups, 4)
				}
			}
			assert.Len(t, started, len(peers), "固定机房和地址集合都应在有限轮次内获得新请求")
		})
	}
}

func TestDataCenterSchedule_ActualAttempts(t *testing.T) {
	r := newLocalReachability()
	var members []*endpoint.Information
	var peers []vivid.ActorRef
	for index := range 3 {
		member := reachableMember(fmt.Sprintf("peer:%d", 10001+index))
		member.DataCenter = fmt.Sprintf("dc-%d", index)
		members, peers = append(members, member), append(peers, member.ActorRef)
	}
	selected := r.selectDataCenterPeers(slices.Clone(peers), members, 2)
	for _, group := range r.dataCenters.groups {
		assert.Zero(t, group.sequence, "仅选中不算已开始请求")
	}
	now := time.Now()
	slow := selected[0]
	slowSequence := r.begin(slow, members, now, time.Second, 2)
	fast := selected[1]
	sequence := r.begin(fast, members, now, time.Second, 2)
	r.accept(fast, sequence, fast.GetAddress(), now, time.Second)
	deadline := r.pending[slow.GetAddress()].deadline
	selected = r.selectDataCenterPeers(slices.Clone(peers), members, 2)
	require.Len(t, selected, 2, "只剩一个观察额度时仍保留正常 Gossip")
	assert.NotEqual(t, slow.GetAddress(), selected[0].GetAddress())
	assert.NotEqual(t, fast.GetAddress(), selected[0].GetAddress(), "新机房先于已经开始观察的机房")
	sequence = r.begin(selected[0], members, now, time.Second, 2)
	require.NotZero(t, sequence)
	for range 3 {
		selected = r.selectDataCenterPeers(slices.Clone(peers), members, 2)
		require.Len(t, selected, 2, "观察满额不阻止成员传播")
		assert.Empty(t, r.candidates)
		assert.Equal(t, slowSequence, r.pending[slow.GetAddress()].sequence)
		assert.Equal(t, deadline, r.pending[slow.GetAddress()].deadline)
	}
	r.accept(slow, slowSequence, slow.GetAddress(), now, time.Second)
	selected = r.selectDataCenterPeers(slices.Clone(peers), members, 2)
	assert.Equal(t, slow.GetAddress(), selected[0].GetAddress(), "机房次序只按实际新建请求更新，等待和普通发送不能续期")
}

func TestDataCenterSchedule_AddressDeclarations(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		r := newLocalReachability()
		var members []*endpoint.Information
		var peers []vivid.ActorRef
		for index, labels := range [][]string{{"dc-a", "dc-a"}, {"dc-a", "dc-b", "dc-b"}, {"", "dc-a"}, {"dc-a", "dc-b"}} {
			for history, label := range labels {
				member := reachableMember(fmt.Sprintf("peer:%d", 10001+index))
				member.DataCenter = label
				if index == 3 && history == 1 {
					member.Status = endpoint.StatusRemoved
				}
				members = append(members, member)
				if history == 0 {
					peers = append(peers, member.ActorRef)
				}
			}
		}
		seed := &tombstoneRef{address: "seed:20001"}
		peers = append(peers, seed)
		if reverse {
			slices.Reverse(members)
		}
		r.selectDataCenterPeers(peers, members, 2)
		assert.Equal(t, "dc-a", r.dataCenters.addresses["peer:10001"])
		assert.Empty(t, r.dataCenters.addresses["peer:10002"], "同地址活跃历史冲突时归入未知组，插入次序不能决定机房")
		assert.Empty(t, r.dataCenters.addresses["peer:10003"], "已声明与未声明历史也存在歧义")
		assert.Equal(t, "dc-a", r.dataCenters.addresses["peer:10004"], "退出墓碑不影响可联络实例的机房分类")
		assert.Empty(t, r.dataCenters.addresses[seed.GetAddress()])
	}
}

func TestDataCenterSchedule_Churn(t *testing.T) {
	r := newLocalReachability()
	now := time.Now()
	member := reachableMember("peer:10001")
	member.DataCenter = "dc-a"
	r.selectDataCenterPeers([]vivid.ActorRef{member.ActorRef}, []*endpoint.Information{member}, 1)
	sequence := r.begin(member.ActorRef, nil, now, time.Second, 1)
	assert.NotZero(t, r.dataCenters.groups["dc-a"].sequence)
	// 同地址改由其他机房的新实例回应后，地址轮换历史不重置。
	next := reachableMember(member.ActorRef.GetAddress())
	next.DataCenter = "dc-b"
	r.invalidateDataCenters()
	r.selectDataCenterPeers([]vivid.ActorRef{next.ActorRef}, []*endpoint.Information{next}, 1)
	assert.NotContains(t, r.dataCenters.groups, "dc-a")
	assert.Equal(t, sequence, r.attempts[next.ActorRef.GetAddress()].sequence)
	assert.Equal(t, sequence, r.pending[next.ActorRef.GetAddress()].sequence)
	r.invalidateDataCenters()
	r.selectDataCenterPeers(nil, nil, 1)
	assert.Empty(t, r.dataCenters.groups)
	assert.Empty(t, r.dataCenters.addresses)
	assert.Empty(t, r.pending)
	assert.Empty(t, r.attempts)
	assert.Empty(t, r.selectDataCenterPeers([]vivid.ActorRef{next.ActorRef}, nil, 0))
	assert.Empty(t, r.selectDataCenterPeers([]vivid.ActorRef{next.ActorRef}, nil, -1))
	r.reset()
	assert.Nil(t, r.dataCenters, "停止时释放机房调度状态")
}

func TestActor_GossipDataCenterScheduling(t *testing.T) {
	for _, declared := range []bool{false, true} {
		t.Run(fmt.Sprint(declared), func(t *testing.T) {
			local := reachableMember("local:10001")
			if declared {
				local.DataCenter = "dc-a"
			}
			a := &Actor{info: local, opts: vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterGossipPeersCount(1)), view: NewClusterView(log.GetDefault()), reachability: newLocalReachability()}
			a.view.Members().Upsert(local)
			labels := make(map[string]string)
			for index := range 11 {
				member := reachableMember(fmt.Sprintf("peer:%d", 10002+index))
				member.DataCenter = "dc-a"
				if index == 10 {
					member.DataCenter = "dc-b"
				}
				a.view.Members().Upsert(member)
				labels[member.ActorRef.GetAddress()] = member.DataCenter
			}
			seed := &tombstoneRef{address: "unknown:20001"}
			a.seeds = []vivid.ActorRef{seed, seed, local.ActorRef}
			labels[seed.GetAddress()] = ""
			ctx := &scheduleContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}}
			seen := make(map[string]bool)
			for range 3 {
				ctx.sent = nil
				a.onSpreadGossip(ctx, gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC))
				require.Len(t, ctx.sent, 1)
				send := ctx.sent[0]
				require.NotZero(t, send.ping.ProbeSequence)
				if declared {
					label := labels[send.peer.GetAddress()]
					assert.False(t, seen[label], "实际 Gossip 发送应先给每个机房及未知种子一次新请求机会")
					seen[label] = true
				}
				a.reachability.accept(send.peer, send.ping.ProbeSequence, send.peer.GetAddress(), time.Now(), time.Second)
			}
			if declared {
				assert.Len(t, seen, 3)
			} else {
				assert.Nil(t, a.reachability.dataCenters, "未声明本地机房时保留全体地址轮换")
			}
			a.stopping = true
			a.info.Status = endpoint.StatusLeaving
			a.reachability.reset()
			ctx.sent = nil
			a.onSpreadGossip(ctx, gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCriticalStateChange))
			require.Len(t, ctx.sent, 1)
			assert.NotEqual(t, seed.GetAddress(), ctx.sent[0].peer.GetAddress(), "退出不再联络未知种子")
			assert.Zero(t, ctx.sent[0].ping.ProbeSequence)
			assert.Nil(t, a.reachability.dataCenters)
		})
	}
}

func TestActor_GossipDataCenterScheduleInvalidation(t *testing.T) {
	a, _ := newViewStableActor(endpoint.StatusUp)
	a.info.DataCenter = "dc-local"
	old := reachableMember("peer:10002")
	old.DataCenter = "dc-a"
	a.view.Members().Upsert(old)
	a.view.Version().Update(old.ID(), 1)
	a.reachability = newLocalReachability()
	ctx := &scheduleContext{tombstoneContext: &tombstoneContext{ref: a.info.ActorRef}}
	spread := gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC)
	a.onSpreadGossip(ctx, spread)
	assert.Equal(t, "dc-a", a.reachability.dataCenters.addresses[old.ActorRef.GetAddress()])
	assert.False(t, a.reachability.dataCenters.dirty)
	a.reachability.observe(old.ID(), vivid.ClusterReachabilityReachable, time.Now(), time.Hour)
	a.publishReachabilityChanged(ctx)
	assert.False(t, a.reachability.dataCenters.dirty, "仅可达性变化无需重建成员机房分类")
	// 被拒绝的声明不能影响已有分类或使重复消息反复重建缓存。
	invalid := old.Clone()
	invalid.DataCenter = "dc-b"
	version := a.view.Version().Clone()
	version.Update(old.ID(), 2)
	a.onPong(ctx, gossipmessages.NewPong(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, invalid, a.view.Members().Clone(), version))
	assert.False(t, a.reachability.dataCenters.dirty)
	assert.Equal(t, "dc-a", a.reachability.dataCenters.addresses[old.ActorRef.GetAddress()])
	// 同地址新实例进入视图后必须重新分类，两个活跃声明冲突时不猜测谁更新。
	next := reachableMember(old.ActorRef.GetAddress())
	next.DataCenter = "dc-b"
	version = a.view.Version().Clone()
	version.Update(next.ID(), 1)
	a.onPong(ctx, gossipmessages.NewPong(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, next, a.view.Members().Clone(), version))
	assert.True(t, a.reachability.dataCenters.dirty)
	a.onSpreadGossip(ctx, spread)
	assert.Empty(t, a.reachability.dataCenters.addresses[old.ActorRef.GetAddress()])
	assert.False(t, a.reachability.dataCenters.dirty)
	// 旧实例正常退出后，墓碑不再遮蔽新实例的机房声明。
	removed := old.Clone()
	removed.Status = endpoint.StatusRemoved
	version = a.view.Version().Clone()
	version.Update(old.ID(), 2)
	a.onPong(ctx, gossipmessages.NewPong(a.opts.Name, gossipmessages.GossipTypeCycleSYNC, removed, a.view.Members().Clone(), version))
	assert.True(t, a.reachability.dataCenters.dirty)
	a.onSpreadGossip(ctx, spread)
	assert.Equal(t, "dc-b", a.reachability.dataCenters.addresses[old.ActorRef.GetAddress()])
	assert.Equal(t, endpoint.StatusRemoved, a.view.Members().Get(old.ID()).Status)
	assert.NotContains(t, a.reachability.dataCenters.groups, "dc-a", "失去候选地址的机房释放调度记录")
}
