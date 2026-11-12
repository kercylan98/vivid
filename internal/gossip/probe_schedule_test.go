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

func TestProbeSchedule_Coverage(t *testing.T) {
	for _, limit := range []int{1, 2, 5, 20} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			r := newLocalReachability()
			var peers []vivid.ActorRef
			for i := range 11 {
				peers = append(peers, &tombstoneRef{address: fmt.Sprintf("peer:%d", 10001+i)})
			}
			now := time.Now()
			started := make(map[string]int)
			for round := range (len(peers) + limit - 1) / limit {
				// 完全失败也应在有限批次内覆盖固定集合；输入顺序不能改变已开始的先后次序。
				candidates := slices.Clone(peers)
				if round%2 == 1 {
					slices.Reverse(candidates)
				}
				selected := r.selectPeers(candidates, limit)
				require.Len(t, selected, min(limit, len(peers)))
				for _, peer := range selected {
					sequence := r.begin(peer, nil, now, time.Second, limit)
					require.NotZero(t, sequence)
					started[peer.GetAddress()]++
				}
				assert.LessOrEqual(t, len(r.pending), limit)
				now = now.Add(time.Second)
				r.expire(now, time.Second)
			}
			require.Len(t, started, len(peers), "所有地址均应有开始请求的机会，不以收到响应作为轮换条件")
			for _, count := range started {
				assert.LessOrEqual(t, count, 2)
			}
		})
	}
}

func TestProbeSchedule_SlowPeerDoesNotStarveOthers(t *testing.T) {
	r := newLocalReachability()
	var peers []vivid.ActorRef
	for i := range 9 {
		peers = append(peers, &tombstoneRef{address: fmt.Sprintf("peer:%d", 10001+i)})
	}
	now := time.Now()
	const limit = 2
	first := r.selectPeers(slices.Clone(peers), limit)
	require.Equal(t, peers[:limit], first)
	slowSequence := r.begin(first[0], nil, now, time.Second, limit)
	slowDeadline := now.Add(time.Second)
	fastSequence := r.begin(first[1], nil, now, time.Second, limit)
	r.accept(first[1], fastSequence, "fast", now, time.Second)
	started := map[string]bool{first[0].GetAddress(): true, first[1].GetAddress(): true}
	for range len(peers) - limit {
		now = now.Add(time.Millisecond)
		selected := r.selectPeers(slices.Clone(peers), limit)
		require.Len(t, selected, limit, "慢目标等待期间也保留普通 Gossip 名额")
		peer := selected[0]
		require.False(t, started[peer.GetAddress()], "空闲额度不能反复交给已成功的同一个地址")
		sequence := r.begin(peer, nil, now, time.Second, limit)
		require.NotZero(t, sequence)
		r.accept(peer, sequence, peer.GetAddress(), now, time.Second)
		started[peer.GetAddress()] = true
		assert.Len(t, r.pending, 1)
		assert.Equal(t, slowSequence, r.pending[first[0].GetAddress()].sequence)
		assert.Equal(t, slowDeadline, r.pending[first[0].GetAddress()].deadline)
	}
	assert.Len(t, started, len(peers))
	// 失败地址完成本次预算后仍有重试机会，并不被健康成员永久排除。
	now = now.Add(time.Second)
	r.expire(now, time.Second)
	next := r.selectPeers(slices.Clone(peers), limit)
	assert.Equal(t, first[0].GetAddress(), next[0].GetAddress())
}

func TestProbeSchedule_Churn(t *testing.T) {
	r := newLocalReachability()
	now := time.Now()
	old := reachableMember("old:10001")
	seed := &tombstoneRef{address: "seed:10002"}
	r.selectPeers([]vivid.ActorRef{old.ActorRef}, 1)
	oldSequence := r.begin(old.ActorRef, []*endpoint.Information{old}, now, time.Second, 1)
	// 退出地址不再是候选项，应该立即释放空请求与排序记录。
	selected := r.selectPeers([]vivid.ActorRef{seed}, 1)
	require.Equal(t, []vivid.ActorRef{seed}, selected)
	assert.Empty(t, r.pending)
	assert.Empty(t, r.attempts)
	sequence := r.begin(seed, nil, now, time.Second, 1)
	require.NotZero(t, sequence)
	r.accept(old.ActorRef, oldSequence, old.ID(), now, time.Second)
	assert.Equal(t, vivid.ClusterReachabilityUnknown, r.state(old.ID()))
	r.accept(seed, sequence, "seed-instance", now, time.Second)
	// 相同地址的实例重启不应新增地址排序记录，也不按实例随机 ID 重排。
	current := reachableMember(seed.GetAddress())
	selected = r.selectPeers([]vivid.ActorRef{current.ActorRef}, 1)
	require.Len(t, selected, 1)
	assert.Len(t, r.attempts, 1)
	assert.Equal(t, sequence, r.attempts[seed.GetAddress()].sequence)
	r.selectPeers(nil, 1)
	assert.Empty(t, r.attempts)
	assert.Empty(t, r.pending)
	assert.Empty(t, r.selectPeers([]vivid.ActorRef{seed}, 0))
	assert.Empty(t, r.selectPeers([]vivid.ActorRef{seed}, -1))
	r.selectPeers([]vivid.ActorRef{seed}, 1)
	r.begin(seed, nil, now, time.Second, 1)
	r.reset()
	assert.Empty(t, r.attempts)
	assert.Empty(t, r.pending)
	assert.Nil(t, r.candidates, "停止时释放调度候选缓冲")
}

func TestProbeSchedule_PriorityAndPermutation(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	for range 200 {
		r := newLocalReachability()
		limit := random.IntN(8) + 1
		var peers []vivid.ActorRef
		var eligible []uint64
		for index := range random.IntN(20) + 1 {
			peer := &tombstoneRef{address: fmt.Sprintf("peer:%d", 10001+index)}
			peers = append(peers, peer)
			// 保留未尝试地址和任意次序的历史，检查交换候选时不会遗漏或重复引用。
			sequence := uint64(random.IntN(30))
			r.attempts[peer.GetAddress()] = probeAttempt{sequence: sequence}
			if len(r.pending) < limit && random.IntN(3) == 0 {
				r.pending[peer.GetAddress()] = reachabilityProbe{sequence: sequence}
			} else {
				eligible = append(eligible, sequence)
			}
		}
		random.Shuffle(len(peers), func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })
		before := slices.Clone(peers)
		selected := r.selectPeers(peers, limit)
		assert.ElementsMatch(t, before, peers, "候选交换不能丢失或重复目标")
		require.Len(t, selected, min(limit, len(peers)))
		available := min(len(selected), max(limit-len(r.pending), 0), len(eligible))
		slices.Sort(eligible)
		for index := range available {
			address := selected[index].GetAddress()
			_, waiting := r.pending[address]
			assert.False(t, waiting)
			assert.Equal(t, eligible[index], r.attempts[address].sequence, "新额度应按实际开始请求的先后分配")
		}
		assert.LessOrEqual(t, len(r.candidates), limit)
	}
}

type scheduledSend struct {
	peer vivid.ActorRef
	ping *gossipmessages.Ping
}

type scheduleContext struct {
	*tombstoneContext
	sent []scheduledSend
}

func (c *scheduleContext) Tell(peer vivid.ActorRef, message vivid.Message) {
	if ping, ok := message.(*gossipmessages.Ping); ok {
		c.sent = append(c.sent, scheduledSend{peer: peer, ping: ping})
	}
}

func TestActor_GossipProbeScheduling(t *testing.T) {
	local := reachableMember("local:10001")
	opts := vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterGossipPeersCount(2), vivid.WithActorSystemClusterProbeTimeout(time.Hour))
	a := &Actor{info: local, opts: opts, view: NewClusterView(log.GetDefault()), reachability: newLocalReachability()}
	a.view.Members().Upsert(local)
	for i := range 7 {
		a.view.Members().Upsert(reachableMember(fmt.Sprintf("peer:%d", 10002+i)))
	}
	// 未知种子必须参与公平轮换，但不能直接成为成员。
	a.seeds = []vivid.ActorRef{&tombstoneRef{address: "unknown:20001"}}
	ctx := &scheduleContext{tombstoneContext: &tombstoneContext{ref: local.ActorRef}}
	started := make(map[string]uint64)
	for range 4 {
		ctx.sent = nil
		a.onSpreadGossip(ctx, gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC))
		require.Len(t, ctx.sent, 2)
		for _, send := range ctx.sent {
			address := send.peer.GetAddress()
			require.NotZero(t, send.ping.ProbeSequence)
			require.Zero(t, started[address], "全体地址有机会前不重复占用新额度")
			started[address] = send.ping.ProbeSequence
		}
		// 不应答时仍发送正常 Gossip，且等待请求不会因多次发送重置。
		ctx.sent = nil
		a.onSpreadGossip(ctx, gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC))
		require.Len(t, ctx.sent, 2)
		for address, probe := range a.reachability.pending {
			assert.Equal(t, started[address], probe.sequence)
		}
		a.reachability.expire(time.Now().Add(2*time.Hour), time.Second)
	}
	assert.Len(t, started, 8)
	assert.Contains(t, started, a.seeds[0].GetAddress())
	assert.Len(t, a.view.Members().List(), 8)
}
