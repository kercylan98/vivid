package ves

import "github.com/kercylan98/vivid"

// ClusterMembersChangedEvent 在 members 发生变更时发布到 EventStream（新增或故障剔除）。
type ClusterMembersChangedEvent struct {
	NodeRef    vivid.ActorRef   // 变更的节点引用
	Members    []vivid.ActorRef // 变更后的成员列表
	AddedNum   []vivid.ActorRef // 新增的成员列表
	RemovedNum []vivid.ActorRef // 移除的成员列表
}
