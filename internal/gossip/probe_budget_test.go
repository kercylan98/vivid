package gossip

import (
	"fmt"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProbeBudgetActor(sendLimit, probeLimit int, declared bool) (*Actor, *scheduleContext) {
	local := reachableMember("local:10001")
	if declared {
		local.DataCenter = "dc-local"
	}
	a := &Actor{
		info: local,
		opts: vivid.NewActorSystemClusterOptions(
			vivid.WithActorSystemClusterName("vivid-test"),
			vivid.WithActorSystemClusterGossipPeersCount(sendLimit),
			vivid.WithActorSystemClusterMaxConcurrentProbes(probeLimit),
			vivid.WithActorSystemClusterProbeTimeout(time.Hour),
		),
		view: NewClusterView(log.GetDefault()), reachability: newLocalReachability(),
	}
	a.view.Members().Upsert(local)
	for index := range 6 {
		member := reachableMember(fmt.Sprintf("peer:%d", 10002+index))
		member.DataCenter = "dc-local"
		if index >= 3 {
			member.DataCenter = "dc-remote"
		}
		a.view.Members().Upsert(member)
	}
	a.seeds = []vivid.ActorRef{&tombstoneRef{address: "unknown:20001"}}
	return a, &scheduleContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}}
}

func TestActor_GossipProbeIndependentBudgets(t *testing.T) {
	for _, declared := range []bool{false, true} {
		for _, budget := range []struct{ send, probe int }{{1, 3}, {3, 1}, {3, 5}, {5, 2}, {9, 12}} {
			t.Run(fmt.Sprintf("declared_%t/send_%d/probe_%d", declared, budget.send, budget.probe), func(t *testing.T) {
				a, ctx := newProbeBudgetActor(budget.send, budget.probe, declared)
				spread := gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC)
				waiting := make(map[string]reachabilityProbe)
				for round := range 7 {
					ctx.sent = nil
					a.onSpreadGossip(ctx, spread)
					require.Len(t, ctx.sent, min(budget.send, 7), "即使观察额度已满，也按原发送额度交换完整视图")
					require.Len(t, a.reachability.pending, min((round+1)*budget.send, budget.probe, 7), "观察并发可跨多轮积累，且不依赖单轮发送上限")
					for _, send := range ctx.sent {
						assert.Len(t, send.ping.MemberList.List(), 7, "关联额度不影响成员视图传播")
					}
					for address, probe := range a.reachability.pending {
						if old, exists := waiting[address]; exists {
							assert.Equal(t, old.sequence, probe.sequence)
							assert.Equal(t, old.deadline, probe.deadline, "普通发送或重发不能重置慢请求的预算")
						}
						waiting[address] = probe
					}
				}
				// 一个有效应答只释放自己的名额，下一轮可建立一个新请求。
				for address, probe := range a.reachability.pending {
					a.reachability.accept(&tombstoneRef{address: address}, probe.sequence, "remote-instance", time.Now(), time.Second)
					break
				}
				before := a.reachability.sequence
				ctx.sent = nil
				a.onSpreadGossip(ctx, spread)
				assert.Greater(t, a.reachability.sequence, before)
				assert.Len(t, a.reachability.pending, min(budget.probe, 7))
				assert.Len(t, ctx.sent, min(budget.send, 7))
				a.reachability.expire(time.Now().Add(2*time.Hour), time.Second)
				assert.Empty(t, a.reachability.pending)
				ctx.sent = nil
				a.onSpreadGossip(ctx, spread)
				assert.Len(t, a.reachability.pending, min(budget.probe, budget.send, 7), "超时后重新获得名额")
				// 退出不再保留或创建观察，也不继续探测未知种子。
				a.stopping, a.info.Status = true, endpoint.StatusLeaving
				a.reachability.reset()
				ctx.sent = nil
				a.onSpreadGossip(ctx, spread)
				assert.Empty(t, a.reachability.pending)
				require.Len(t, ctx.sent, min(budget.send, 6))
				for _, send := range ctx.sent {
					assert.Zero(t, send.ping.ProbeSequence)
					assert.NotEqual(t, a.seeds[0].GetAddress(), send.peer.GetAddress())
				}
			})
		}
	}
}

func TestActor_GossipProbeSlowAddress(t *testing.T) {
	for _, declared := range []bool{false, true} {
		t.Run(fmt.Sprint(declared), func(t *testing.T) {
			a, ctx := newProbeBudgetActor(1, 2, declared)
			spread := gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC)
			a.onSpreadGossip(ctx, spread)
			require.Len(t, ctx.sent, 1)
			slow := ctx.sent[0].peer.GetAddress()
			initial := a.reachability.pending[slow]
			started := map[string]bool{slow: true}
			for range 20 {
				ctx.sent = nil
				a.onSpreadGossip(ctx, spread)
				require.Len(t, ctx.sent, 1)
				send := ctx.sent[0]
				assert.NotEqual(t, slow, send.peer.GetAddress(), "单轮只发送一个地址时，慢目标不应阻止剩余并发额度轮换")
				started[send.peer.GetAddress()] = true
				a.reachability.accept(send.peer, send.ping.ProbeSequence, send.peer.GetAddress(), time.Now(), time.Second)
				assert.Len(t, a.reachability.pending, 1)
				assert.Equal(t, initial, a.reachability.pending[slow])
			}
			assert.Len(t, started, 7, "包括未知种子在内的地址都有机会开始请求")
		})
	}
}
