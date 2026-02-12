package handler

import (
	"net/http"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/console/internal/api"
	"github.com/kercylan98/vivid/pkg/metrics"
)

func NewNode(system vivid.ActorSystem) *Node {
	return &Node{System: system}
}

// Node 持有节点相关 HTTP 处理器依赖。
type Node struct {
	System vivid.ActorSystem
}

// CurrentNodeCluster 返回当前节点的 cluster 资源，GET /api/nodes/current/cluster。
func (n *Node) CurrentNodeCluster(w http.ResponseWriter, _ *http.Request) {
	type clusterStatusResponse struct {
		IsClusterNode bool   `json:"isClusterNode"`
		LeaderAddr    string `json:"leaderAddr,omitempty"`
		InQuorum      bool   `json:"inQuorum"`
		MemberCount   int    `json:"memberCount"`
	}

	cluster := n.System.Cluster()
	if cluster == nil {
		api.Ok(w, &clusterStatusResponse{})
		return
	}
	view, err := cluster.GetView()
	if err != nil {
		api.Ok(w, &clusterStatusResponse{IsClusterNode: n.System.IsClusterEnabled()})
		return
	}
	members, _ := cluster.GetMembers()
	resp := &clusterStatusResponse{
		IsClusterNode: true,
		MemberCount:   len(members),
	}
	if view != nil {
		resp.LeaderAddr = view.LeaderAddr
		resp.InQuorum = view.InQuorum
	}
	api.Ok(w, resp)
}

// CurrentNodeState 返回当前节点的系统基本状态，GET /api/nodes/current/state。
// 若 ActorSystem 未实现 SystemStateProvider，则返回零值结构体（前端以占位展示）。
func (n *Node) CurrentNodeState(w http.ResponseWriter, _ *http.Request) {
	if p, ok := n.System.(vivid.SystemStateProvider); ok {
		api.Ok(w, p.GetSystemBasicState())
		return
	}
	api.Ok(w, vivid.SystemBasicState{})
}

// CurrentNodeMetrics 返回当前节点的指标快照，GET /api/nodes/current/metrics。
// 需实现 MetricsProvider 且 MetricsEnabled 为 true 时返回实时快照；否则返回空快照。
func (n *Node) CurrentNodeMetrics(w http.ResponseWriter, _ *http.Request) {
	if mp, ok := n.System.(vivid.MetricsProvider); ok && mp.MetricsEnabled() {
		api.Ok(w, mp.Metrics().Snapshot())
		return
	}
	api.Ok(w, metrics.MetricsSnapshot{})
}
