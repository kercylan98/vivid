package cluster

import (
	"github.com/kercylan98/vivid"
)

// ClusterMetricsUpdater 负责更新集群相关指标（成员数、健康数、quorum、DC 等）。
type ClusterMetricsUpdater struct{}

// NewClusterMetricsUpdater 创建指标更新器。
func NewClusterMetricsUpdater() *ClusterMetricsUpdater {
	return &ClusterMetricsUpdater{}
}

// Update 根据当前视图更新 cluster.* 指标。
func (u *ClusterMetricsUpdater) Update(ctx vivid.ActorContext, v *ClusterView) {
	if ctx == nil || !ctx.MetricsEnabled() || v == nil {
		return
	}
	m := ctx.Metrics()
	m.Gauge("cluster.members").Set(int64(len(v.Members)))
	m.Gauge("cluster.healthy").Set(int64(v.HealthyCount))
	m.Gauge("cluster.unhealthy").Set(int64(v.UnhealthyCount))
	m.Gauge("cluster.quorum_size").Set(int64(v.QuorumSize))
	if v.QuorumSize > 0 && v.HealthyCount >= v.QuorumSize {
		m.Gauge("cluster.in_quorum").Set(1)
	} else {
		m.Gauge("cluster.in_quorum").Set(0)
	}
	dcMembers := make(map[string]int)
	dcHealthy := make(map[string]int)
	for _, node := range v.Members {
		if node == nil {
			continue
		}
		dc := node.Datacenter()
		if dc == "" {
			dc = "_default"
		}
		dcMembers[dc]++
		if node.Status == MemberStatusUp {
			dcHealthy[dc]++
		}
	}
	for dc, count := range dcMembers {
		m.Gauge("cluster.dc." + dc + ".members").Set(int64(count))
		m.Gauge("cluster.dc." + dc + ".healthy").Set(int64(dcHealthy[dc]))
	}
}

// UpdateViewDivergence 更新与另一视图的差异指标。
func (u *ClusterMetricsUpdater) UpdateViewDivergence(ctx vivid.ActorContext, local, other *ClusterView) {
	if ctx == nil || !ctx.MetricsEnabled() || local == nil || other == nil {
		return
	}
	our := make(map[string]bool)
	for id := range local.Members {
		our[id] = true
	}
	diff := 0
	for id := range other.Members {
		if !our[id] {
			diff++
		}
	}
	for id := range our {
		if other.Members[id] == nil {
			diff++
		}
	}
	ctx.Metrics().Gauge("cluster.view_divergence").Set(int64(diff))
}
