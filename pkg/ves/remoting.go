package ves

// RemotingInboundConnectionEstablishedEvent 表示入站连接建立完成的事件。
//
// 该事件在 Acceptor 完成握手并为对端地址创建或关联 Endpoint、挂上 Session 后发布。
//
// 使用场景：
//   - 统计入站连接数、连接来源
//   - 审计/安全：记录谁连入了本节点
//   - 与负载、限流等策略联动
type RemotingInboundConnectionEstablishedEvent struct {
	// PeerAddress 对端广告地址（或 RemoteAddr）
	PeerAddress string
	// LocalAddress 本端广告地址（或 LocalAddr）
	LocalAddress string
}

// RemotingOutboundConnectionEstablishedEvent 表示出站连接建立完成的事件。
//
// 该事件在向某远程地址发起连接并完成握手、Session 激活后发布。
//
// 使用场景：
//   - 确认到某节点的出站通道已可用
//   - 统计出站连接、重连成功次数
//   - 与集群成员状态、健康检查联动
type RemotingOutboundConnectionEstablishedEvent struct {
	// Address 远程节点广告地址
	Address string
}

// RemotingConnectionFailedEvent 表示连接失败的事件。
//
// 该事件在出站连接失败（Dial 失败、握手失败、Session 激活失败等）或重试耗尽时发布。
// 可通过 EventStream.Subscribe(ctx, ves.RemotingConnectionFailedEvent{}) 订阅。
//
// 使用场景：
//   - 监控远程不可达、网络分区
//   - 告警与运维诊断
//   - 测试中验证向无效地址发送会收到此事件
type RemotingConnectionFailedEvent struct {
	// Address 目标远程地址
	Address string
	// Error 失败原因
	Error error
}

// RemotingConnectionClosedEvent 表示连接关闭的事件。
//
// 该事件在 Session 关闭时发布（读端异常、对端关闭、写失败导致关闭等）。
//
// 使用场景：
//   - 区分正常关闭与异常断开
//   - 统计连接存活时间、断开原因
//   - 触发重连或从集群剔除
type RemotingConnectionClosedEvent struct {
	// Address 对端地址（入站时为 PeerAddress，出站时为目标 Address）
	Address string
	// PeerClosed 是否由对端主动关闭（如读到 EOF/Close 帧）
	PeerClosed bool
	// Error 若因错误关闭，记录错误；否则为 nil
	Error error
}

// RemotingEnvelopSendFailedEvent 表示向远程发送 Envelop 失败的事件。
//
// 该事件在 Remoting 已停止、Endpoint 发送失败（如缓冲区满、写失败导致 HandleFailedRemotingEnvelop）时发布。
//
// 使用场景：
//   - 监控消息投递失败、做死信或重试
//   - 与背压、限流策略配合
type RemotingEnvelopSendFailedEvent struct {
	// TargetAddress 目标远程地址
	TargetAddress string
	// Error 失败原因
	Error error
}
