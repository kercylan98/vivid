package gossip

import "github.com/kercylan98/vivid"

// MembersChangedEvent 成员列表变更事件
type MembersChangedEvent struct {
	NodeRef  vivid.ActorRef   // 变更的节点引用
	Members  []vivid.ActorRef // 变更后的成员列表
	Addeds   []vivid.ActorRef // 新增的成员列表
	Removeds []vivid.ActorRef // 移除的成员列表
}
