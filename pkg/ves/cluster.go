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
