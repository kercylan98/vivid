package cluster

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/ves"
)

// ClusterEventPublisher 管理集群事件发布状态（上次 quorum/leader/DC 健康），并发布视图与成员变更事件。
type ClusterEventPublisher struct {
	lastInQuorum   bool
	lastLeaderAddr string
	lastDCHealth   map[string]bool
}

// NewClusterEventPublisher 创建集群事件发布器。
func NewClusterEventPublisher() *ClusterEventPublisher {
	return &ClusterEventPublisher{
		lastDCHealth: make(map[string]bool),
	}
}

// PublishMembersChanged 发布成员变更事件。
func (p *ClusterEventPublisher) PublishMembersChanged(ctx vivid.ActorContext, members []string, addedNum int, removed []string) {
	if ctx == nil {
		return
	}
	es := ctx.EventStream()
	if es == nil {
		return
	}
	removedNum := len(removed)
	es.Publish(ctx, ves.ClusterMembersChangedEvent{
		NodeRef:    ctx.Ref(),
		Members:    members,
		AddedNum:   addedNum,
		RemovedNum: removedNum,
		Removed:    removed,
	})
}

// PublishViewChanged 发布视图变更事件。
func (p *ClusterEventPublisher) PublishViewChanged(ctx vivid.ActorContext, v *ClusterView, addedNum int, removed []string) {
	if ctx == nil || v == nil {
		return
	}
	es := ctx.EventStream()
	if es == nil {
		return
	}
	es.Publish(ctx, ves.ClusterViewChangedEvent{
		NodeRef:        ctx.Ref(),
		HealthyCount:   v.HealthyCount,
		UnhealthyCount: v.UnhealthyCount,
		QuorumSize:     v.QuorumSize,
		MemberCount:    len(v.Members),
		AddedNum:       addedNum,
		RemovedNum:     len(removed),
		Removed:        removed,
	})
}

// PublishDCHealthChangedIfNeeded 在 DC 健康状态变化时发布事件。
func (p *ClusterEventPublisher) PublishDCHealthChangedIfNeeded(ctx vivid.ActorContext, v *ClusterView) {
	if ctx == nil || v == nil {
		return
	}
	es := ctx.EventStream()
	if es == nil {
		return
	}
	dcHealthy := make(map[string]int)
	dcTotal := make(map[string]int)
	for _, m := range v.Members {
		if m == nil {
			continue
		}
		dc := m.Datacenter()
		if dc == "" {
			dc = "_default"
		}
		dcTotal[dc]++
		if m.Status == MemberStatusUp {
			dcHealthy[dc]++
		}
	}
	for dc := range dcTotal {
		isHealthy := dcHealthy[dc] >= 1
		if was, ok := p.lastDCHealth[dc]; !ok || was != isHealthy {
			es.Publish(ctx, ves.ClusterDCHealthChangedEvent{
				NodeRef:        ctx.Ref(),
				Datacenter:     dc,
				HealthyCount:   dcHealthy[dc],
				UnhealthyCount: dcTotal[dc] - dcHealthy[dc],
				MemberCount:    dcTotal[dc],
				IsHealthy:      isHealthy,
			})
			p.lastDCHealth[dc] = isHealthy
		}
	}
	for dc := range p.lastDCHealth {
		if _, exists := dcTotal[dc]; !exists {
			delete(p.lastDCHealth, dc)
		}
	}
}

// PublishLeaderIfChanged 在 quorum 或确定性 Leader 变化时发布 QuorumLost/QuorumReached 与 ClusterLeaderChangedEvent。
func (p *ClusterEventPublisher) PublishLeaderIfChanged(ctx vivid.ActorContext, v *ClusterView, selfAddr string, inQuorum bool) {
	if ctx == nil || v == nil {
		return
	}
	es := ctx.EventStream()
	if es == nil {
		return
	}
	leaderAddr := ComputeLeaderAddr(v)
	if p.lastInQuorum && !inQuorum {
		es.Publish(ctx, ves.ClusterQuorumLostEvent{
			NodeRef:        ctx.Ref(),
			HealthyCount:   v.HealthyCount,
			QuorumSize:     v.QuorumSize,
			UnhealthyCount: v.UnhealthyCount,
		})
	} else if !p.lastInQuorum && inQuorum {
		es.Publish(ctx, ves.ClusterQuorumReachedEvent{
			NodeRef:      ctx.Ref(),
			HealthyCount: v.HealthyCount,
			QuorumSize:   v.QuorumSize,
		})
	}
	if leaderAddr != p.lastLeaderAddr || inQuorum != p.lastInQuorum {
		es.Publish(ctx, ves.ClusterLeaderChangedEvent{
			NodeRef:    ctx.Ref(),
			LeaderAddr: leaderAddr,
			IAmLeader:  selfAddr != "" && leaderAddr == selfAddr,
			InQuorum:   inQuorum,
		})
		p.lastLeaderAddr = leaderAddr
	}
	p.lastInQuorum = inQuorum
}
