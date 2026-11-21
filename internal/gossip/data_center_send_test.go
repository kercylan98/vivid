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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataCenterSchedule_OrdinarySends(t *testing.T) {
	for _, limit := range []int{1, 2, 5, 200} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			r := newLocalReachability()
			var members []*endpoint.Information
			var peers []vivid.ActorRef
			labels := make(map[string]string)
			counts := map[string]int{"dc-a": 90, "dc-b": 8, "dc-c": 1, "": 1}
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
			random := rand.New(rand.NewPCG(3, 4))
			seen := make(map[string]map[string]bool)
			groups := make(map[string]bool)
			for range (4*90 + limit - 1) / limit {
				candidates := slices.Clone(peers)
				random.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
				// 内部零空闲预算用于隔离普通发送，不创建新的观察。
				selected := r.selectDataCenterPeers(candidates, memberRecords(members), limit, 0)
				require.Len(t, selected, min(limit, len(peers)))
				assert.ElementsMatch(t, peers, candidates, "重排不能重复或丢失目标")
				for _, peer := range selected {
					address := peer.GetAddress()
					label := labels[address]
					if limit <= 4 {
						require.False(t, groups[label], "普通发送也要让小机房和未知组轮到，不按地址数量分配机房机会")
						groups[label] = true
						if len(groups) == 4 {
							clear(groups)
						}
					}
					if seen[label] == nil {
						seen[label] = make(map[string]bool)
					}
					if len(seen[label]) < counts[label] {
						require.False(t, seen[label][address], "机房内按普通发送历史轮换地址")
					}
					seen[label][address] = true
					r.recordDataCenterSend(peer)
				}
			}
			for label, count := range counts {
				assert.Len(t, seen[label], count)
				assert.Zero(t, r.dataCenters.groups[label].sequence, "普通发送不能推进新观察的机房历史")
			}
			for _, attempt := range r.attempts {
				assert.Zero(t, attempt.sequence, "普通发送不能推进地址的新观察历史")
			}
			assert.Zero(t, r.sequence)
			assert.Empty(t, r.pending)
		})
	}
}

func TestDataCenterSchedule_SendHistory(t *testing.T) {
	r := newLocalReachability()
	member := reachableMember("peer:10001")
	member.DataCenter = "dc-a"
	peer, members := []vivid.ActorRef{member.ActorRef}, []*endpoint.Information{member}
	r.selectDataCenterPeers(peer, memberRecords(members), 1, 0)
	assert.Zero(t, r.dataCenters.sequence, "仅选择不算已经发送")
	r.recordDataCenterSend(member.ActorRef)
	sent := r.attempts[member.ActorRef.GetAddress()].sent
	require.NotZero(t, sent)
	sequence := r.begin(member.ActorRef, memberRecords(members), time.Now(), time.Hour, 1)
	assert.Equal(t, sent, r.attempts[member.ActorRef.GetAddress()].sent, "新请求不能覆盖普通发送历史")
	deadline := r.pending[member.ActorRef.GetAddress()].deadline
	r.selectDataCenterPeers(peer, memberRecords(members), 1, 1)
	r.recordDataCenterSend(member.ActorRef)
	assert.Equal(t, sequence, r.attempts[member.ActorRef.GetAddress()].sequence)
	assert.Equal(t, sequence, r.dataCenters.groups["dc-a"].sequence)
	assert.Equal(t, deadline, r.pending[member.ActorRef.GetAddress()].deadline, "重发不能延长观察预算")
	// 同地址重新分类保留地址发送历史，机房记录则按当前候选清理。
	next := reachableMember(member.ActorRef.GetAddress())
	next.DataCenter = "dc-b"
	r.invalidateDataCenters()
	r.selectDataCenterPeers([]vivid.ActorRef{next.ActorRef}, memberRecords([]*endpoint.Information{next}), 1, 1)
	assert.NotContains(t, r.dataCenters.groups, "dc-a")
	assert.Greater(t, r.attempts[next.ActorRef.GetAddress()].sent, sent)
	r.invalidateDataCenters()
	r.selectDataCenterPeers(nil, nil, 1, 1)
	assert.Empty(t, r.attempts, "地址退出后也释放发送历史")
	assert.Empty(t, r.dataCenters.groups)
	r.reset()
	assert.Nil(t, r.dataCenters)
}

func TestActor_GossipDataCenterOrdinarySends(t *testing.T) {
	for _, sendLimit := range []int{1, 3, 5} {
		t.Run(fmt.Sprint(sendLimit), func(t *testing.T) {
			a, ctx := newProbeBudgetActor(sendLimit, 1, true)
			spread := gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC)
			a.onSpreadGossip(ctx, spread)
			require.Len(t, a.reachability.pending, 1)
			waiting := make(map[string]reachabilityProbe)
			for address, probe := range a.reachability.pending {
				waiting[address] = probe
			}
			started := a.reachability.sequence
			seen := make(map[string]bool)
			groups := make(map[string]bool)
			for range 18 {
				ctx.sent = nil
				a.onSpreadGossip(ctx, spread)
				require.Len(t, ctx.sent, sendLimit)
				round := make(map[string]bool)
				roundGroups := make(map[string]bool)
				for _, send := range ctx.sent {
					address := send.peer.GetAddress()
					assert.False(t, round[address], "本轮按地址去重")
					round[address], seen[address] = true, true
					groups[a.reachability.dataCenters.addresses[address]] = true
					roundGroups[a.reachability.dataCenters.addresses[address]] = true
					assert.Len(t, send.ping.MemberList.List(), 7)
					if send.ping.ProbeSequence != 0 {
						assert.Equal(t, waiting[address].sequence, send.ping.ProbeSequence, "等待请求沿用原序号")
					}
				}
				assert.Equal(t, waiting, a.reachability.pending)
				assert.Equal(t, started, a.reachability.sequence, "满额普通发送不冒充新观察")
				if sendLimit >= 3 {
					assert.Len(t, roundGroups, 3, "单轮有足够发送名额时先联系各机房和未知种子组")
				}
			}
			assert.Len(t, seen, 7, "观察满额时也能轮换全部地址和未知种子")
			assert.Len(t, groups, 3)
		})
	}
}

func TestActor_GossipDataCenterMixedSends(t *testing.T) {
	a, ctx := newProbeBudgetActor(3, 1, true)
	a.onSpreadGossip(ctx, gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC))
	require.Len(t, ctx.sent, 3)
	require.Len(t, a.reachability.pending, 1)
	groups := make(map[string]bool)
	for index, send := range ctx.sent {
		address := send.peer.GetAddress()
		group := a.reachability.dataCenters.addresses[address]
		require.False(t, groups[group], "新观察的机房已占用发送名额，普通发送先联系其他机房")
		groups[group] = true
		assert.Equal(t, uint64(index+1), a.reachability.attempts[address].sent)
		if index == 0 {
			assert.NotZero(t, send.ping.ProbeSequence)
			assert.NotZero(t, a.reachability.attempts[address].sequence)
		} else {
			assert.Zero(t, send.ping.ProbeSequence)
			assert.Zero(t, a.reachability.attempts[address].sequence, "普通发送不占用新观察机会")
			assert.Zero(t, a.reachability.dataCenters.groups[group].sequence)
		}
	}
}
