package cluster

import "github.com/kercylan98/vivid"

// LeaveCoordinator 管理优雅退出流程中 Leave Ask 的回复方，确保在 Exiting 后只回复一次。
type LeaveCoordinator struct {
	replyTo vivid.ActorRef
}

// NewLeaveCoordinator 创建 Leave 协调器。
func NewLeaveCoordinator() *LeaveCoordinator {
	return &LeaveCoordinator{}
}

// SetReplyTo 设置待回复的 Leave 调用方。
func (c *LeaveCoordinator) SetReplyTo(ref vivid.ActorRef) {
	c.replyTo = ref
}

// GetAndClearReplyTo 取出并清空待回复方，用于在 Exiting 后回复 LeaveAck。
func (c *LeaveCoordinator) GetAndClearReplyTo() vivid.ActorRef {
	ref := c.replyTo
	c.replyTo = nil
	return ref
}

// ReplyTo 返回当前待回复方（不修改状态）。
func (c *LeaveCoordinator) ReplyTo() vivid.ActorRef {
	return c.replyTo
}
