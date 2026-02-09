package ves

import "github.com/kercylan98/vivid"

// ClusterMembersChangedEvent 在 members 发生变更时发布到 EventStream（新增或故障剔除）。
type ClusterMembersChangedEvent struct {
	NodeRef    vivid.ActorRef // 变更的节点引用
	Members    []string       // 变更后的成员列表
	AddedNum   int            // 新增的成员数量
	RemovedNum int            // 移除的成员数量
	Removed    []string       // 移除的成员列表
}

// ClusterLeaderChangedEvent 在确定性选主结果变化时发布到 EventStream，便于集群单例等迁移。
// InQuorum 为 false 表示当前视图未达多数派（可能处于分区或少数派），此时不应以 Leader 做关键决策。
type ClusterLeaderChangedEvent struct {
	NodeRef    vivid.ActorRef // 变更的节点引用
	LeaderAddr string         // 新的领导者地址
	IAmLeader  bool           // 当前节点是否为领导者
	InQuorum   bool           // 当前节点是否处于多数派
}

// ClusterQuorumLostEvent 当前节点失去多数派时发布（HealthyCount < QuorumSize）。
type ClusterQuorumLostEvent struct {
	NodeRef             vivid.ActorRef
	HealthyCount        int
	QuorumSize          int
	UnhealthyCount      int
}

// ClusterQuorumReachedEvent 当前节点重新达到多数派时发布。
type ClusterQuorumReachedEvent struct {
	NodeRef      vivid.ActorRef
	HealthyCount int
	QuorumSize   int
}

// ClusterViewChangedEvent 成员视图发生变更时发布（新增、剔除、Suspect 等），便于监控与告警。
type ClusterViewChangedEvent struct {
	NodeRef        vivid.ActorRef
	HealthyCount   int
	UnhealthyCount int
	QuorumSize     int
	MemberCount    int
	AddedNum       int
	RemovedNum     int
	Removed        []string
}

// ClusterDCHealthChangedEvent 某 DC 健康状态变化时发布（从有健康节点变为无，或从无变为有），便于区域级故障感知与切换。
type ClusterDCHealthChangedEvent struct {
	NodeRef         vivid.ActorRef
	Datacenter      string
	HealthyCount    int
	UnhealthyCount  int
	MemberCount     int
	IsHealthy       bool // 当前该 DC 是否至少有一个健康节点
}

// ClusterLeaveCompletedEvent 本节点完成优雅退出流程（已广播离开视图并进入 Exiting、已回复 LeaveAck）时发布。
// 供临时启动的 LeaveWatcher 等监听以检测「已退出」并解除阻塞。
type ClusterLeaveCompletedEvent struct {
	NodeRef vivid.ActorRef
}
