package gossip

import (
	"slices"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/ves"
)

type reachabilityObservation struct {
	state     vivid.ClusterReachability
	expiresAt time.Time
}

type reachabilityProbe struct {
	sequence uint64
	deadline time.Time
	members  []string
}

// localReachability 仅在 Gossip Actor 线程中访问；未完成请求按地址去重且数量有界。
// 时间来自本地 time.Now，保留单调时钟，不比较远端时间戳。
type localReachability struct {
	observations map[string]reachabilityObservation
	pending      map[string]reachabilityProbe
	sequence     uint64
	changed      bool
}

func newLocalReachability() *localReachability {
	return &localReachability{
		observations: make(map[string]reachabilityObservation),
		pending:      make(map[string]reachabilityProbe),
	}
}

func observableMember(status endpoint.Status) bool {
	return status == endpoint.StatusUp || status == endpoint.StatusLeaving || status == endpoint.StatusExiting
}

func (r *localReachability) state(id string) vivid.ClusterReachability {
	if r == nil {
		return vivid.ClusterReachabilityUnknown
	}
	return r.observations[id].state
}

func (r *localReachability) observe(id string, state vivid.ClusterReachability, now time.Time, ttl time.Duration) {
	if r.state(id) != state {
		r.changed = true
	}
	r.observations[id] = reachabilityObservation{state: state, expiresAt: now.Add(ttl)}
}

// begin 为现有 Gossip 消息关联观察；额度耗尽时仍允许正常交换视图。
func (r *localReachability) begin(peer vivid.ActorRef, members []*endpoint.Information, now time.Time, timeout time.Duration, limit int) uint64 {
	key := peer.GetAddress()
	if probe, exists := r.pending[key]; exists {
		return probe.sequence // 重发沿用原序号与截止时间。
	}
	if len(r.pending) >= limit {
		return 0
	}
	r.sequence++
	if r.sequence == 0 {
		r.sequence++
	}
	probe := reachabilityProbe{sequence: r.sequence, deadline: now.Add(timeout)}
	for _, member := range members {
		if member.ActorRef.GetAddress() == key && observableMember(member.Status) {
			probe.members = append(probe.members, member.ID())
		}
	}
	r.pending[key] = probe
	return probe.sequence
}

func (r *localReachability) expire(now time.Time, ttl time.Duration) {
	for key, probe := range r.pending {
		if now.Before(probe.deadline) {
			continue
		}
		// 调度迟滞跨过整个有效期时，不发布已经过期的失败结果。
		if now.Before(probe.deadline.Add(ttl)) {
			for _, id := range probe.members {
				r.observe(id, vivid.ClusterReachabilityUnreachable, probe.deadline, ttl)
			}
		}
		delete(r.pending, key)
	}
	for id, observation := range r.observations {
		if !now.Before(observation.expiresAt) {
			delete(r.observations, id)
			r.changed = true
		}
	}
}

func (r *localReachability) accept(peer vivid.ActorRef, sequence uint64, id string, now time.Time, ttl time.Duration) {
	key := peer.GetAddress()
	probe, exists := r.pending[key]
	if !exists || sequence == 0 || probe.sequence != sequence || !now.Before(probe.deadline) {
		return
	}
	delete(r.pending, key)
	// 同一地址只将本次直接应答的实例标记为可达，不替其他历史实例续期。
	for _, expected := range probe.members {
		if expected != id {
			r.observe(expected, vivid.ClusterReachabilityUnreachable, now, ttl)
		}
	}
	r.observe(id, vivid.ClusterReachabilityReachable, now, ttl)
}

func (r *localReachability) forget(id string) {
	if _, exists := r.observations[id]; exists {
		delete(r.observations, id)
		r.changed = true
	}
	for key, probe := range r.pending {
		probe.members = slices.DeleteFunc(probe.members, func(member string) bool { return member == id })
		r.pending[key] = probe
	}
}

func (r *localReachability) reset() {
	r.changed = len(r.observations) > 0
	clear(r.observations)
	clear(r.pending)
}

func (a *Actor) refreshReachability(ctx vivid.ActorContext, now time.Time) {
	if a.reachability != nil && !a.stopping {
		a.reachability.expire(now, a.opts.ReachabilityTTL)
		a.publishReachabilityChanged(ctx)
	}
}

func (a *Actor) observePong(ctx vivid.ActorContext, pong *gossipmessages.Pong, now time.Time) {
	if a.reachability == nil || a.stopping || pong.ProbeSequence == 0 || pong.ProbeOriginID != a.info.ID() ||
		pong.Info.ID() == a.info.ID() || ctx.Sender() == nil || !ctx.Sender().Equals(pong.Info.ActorRef) {
		return
	}
	member := a.view.Members().Get(pong.Info.ID())
	if member == nil || !observableMember(member.Status) || !member.ActorRef.Equals(pong.Info.ActorRef) ||
		!observableMember(pong.Info.Status) || pong.Version.Get(pong.Info.ID()) == 0 {
		return
	}
	a.reachability.accept(pong.Info.ActorRef, pong.ProbeSequence, pong.Info.ID(), now, a.opts.ReachabilityTTL)
	a.publishReachabilityChanged(ctx)
}

func (a *Actor) publishReachabilityChanged(ctx vivid.ActorContext) {
	if a.reachability == nil || !a.reachability.changed {
		return
	}
	a.reachability.changed = false
	a.viewRevision++
	a.snapshot = nil
	ctx.EventStream().Publish(ctx, ves.ClusterReachabilityChangedEvent{Snapshot: a.currentSnapshot()})
}
