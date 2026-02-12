package cluster

import "time"

// 以下消息类型在序列化时仍使用 "ClusterInternalMessageFor*" 作为 wire 名称以保持兼容。

// StartJoinRequest 启动加入集群请求（当前未使用，保留供扩展）。
type StartJoinRequest struct {
	Seeds []string
}

// JoinRequest 节点向种子发起的加入请求。
type JoinRequest struct {
	NodeState *NodeState // 发送方节点状态
	AuthToken string     // 加入认证令牌，当接收方配置了 JoinSecret 时必填
}

// JoinResponse 种子节点对加入请求的回复，携带当前视图。
type JoinResponse struct {
	View *ClusterView
}

// GossipMessage 携带视图的 Gossip 消息，用于成员视图扩散。
type GossipMessage struct {
	View *ClusterView
}

// GossipTick 触发本 DC 内一轮 Gossip 的定时消息。
type GossipTick struct{}

// GossipCrossDCTick 触发跨 DC 一轮 Gossip 的定时消息。
type GossipCrossDCTick struct{}

// FailureDetectionTick 触发故障检测轮次的定时消息。
type FailureDetectionTick struct{}

// LeaveRequest 请求本节点优雅退出集群（Ask 后等待 LeaveAck）。
type LeaveRequest struct{}

// LeaveAck 本节点完成「广播离开视图并进入 Exiting」后回复给 Leave 调用方。
type LeaveAck struct{}

// ExitingReady 内部消息，表示已进入 Exiting 状态并可回复 LeaveAck。
type ExitingReady struct{}

// LeaveBroadcastRound 优雅退出时多轮广播的轮次消息，Round 从 1 开始。
type LeaveBroadcastRound struct {
	Round int
}

// JoinRetryTick 加入重试定时消息，NextDelay 为本次调度使用的延迟（用于日志/序列化）。
type JoinRetryTick struct {
	NextDelay time.Duration
}

// GetViewRequest 请求当前集群视图（用于 Context.GetMembers/InQuorum 及 quorum 恢复）。
type GetViewRequest struct{}

// GetViewResponse 返回当前视图、是否满足法定人数及当前 Leader 地址。
type GetViewResponse struct {
	View       *ClusterView
	InQuorum   bool
	LeaderAddr string // 当前确定性 Leader 的 Remoting 地址，用于 SingletonRef 等
}

// ForceMemberDown 管理消息：将指定节点强制下线并从视图移除。
type ForceMemberDown struct {
	NodeID     string
	AdminToken string
}

// TriggerViewBroadcast 管理消息：立即触发一轮视图广播（用于运维收敛）。
type TriggerViewBroadcast struct {
	AdminToken string
}

// UpdateNodeStateRequest 请求更新本节点的运行时自定义状态（CustomState），合并后通过 Gossip 传播。
// 仅本节点处理；CustomState 为增量合并，nil 表示不修改，非 nil 的 key 会覆盖或新增。
// Metadata/Labels 适用于固定拓扑等常量，运行时可变状态应使用 CustomState。
type UpdateNodeStateRequest struct {
	CustomState map[string]string // 增量合并到 NodeState.CustomState，nil 表示不修改
}
