package ves

import "github.com/kercylan98/vivid"

// ClusterMembersChangedEvent 在本地接受成员更新或自身状态迁移后发布。
// Snapshot 为变更后的只读快照；重复或迟到的旧版本不触发事件。
type ClusterMembersChangedEvent struct {
	Snapshot vivid.ClusterSnapshot
}
