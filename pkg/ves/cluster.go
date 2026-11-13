package ves

import "github.com/kercylan98/vivid"

// ClusterMembersChangedEvent 在本地接受成员更新或自身状态迁移后发布。
// Snapshot 为变更后的只读快照；重复或迟到的旧版本不触发事件。
type ClusterMembersChangedEvent struct {
	Snapshot vivid.ClusterSnapshot
}

// ClusterReachabilityChangedEvent 在本地直接往返观察变化或过期时发布，不传播到其他节点。
// Snapshot 与成员事件共用 LocalID 和本地 Revision，可按相同规则处理交错结果。
type ClusterReachabilityChangedEvent struct {
	Snapshot vivid.ClusterSnapshot
}
