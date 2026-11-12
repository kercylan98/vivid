package metrics

// 系统
const (
	// MessagesProcessedTotalCounter 已投递并进入 Actor OnReceive 的消息总数（不含系统消息如 OnLaunch/OnKill）。
	MessagesProcessedTotalCounter = "messages_processed_total"
	// StreamEventsTotalCounter 通过 EventStream.Publish 发布的事件总数（生命周期、Watch、Mailbox 等）。
	StreamEventsTotalCounter = "stream_events_total"
	// DeathLetterTotalCounter 投递到死信队列的消息总数。
	DeathLetterTotalCounter = "death_letter_total"
)

// Actor
const (
	// SpawnedActorTotalCounter Actor 创建总次数（每次创建 +1）。
	SpawnedActorTotalCounter = "spawned_actor_total"

	// AliveActorCountGauge 当前存活的 Actor 数量（创建 +1，Kill -1）。
	AliveActorCountGauge = "alive_actor_count"

	// ActorLaunchDurationHistogram 从 Spawned 到 Launched 的耗时（秒），反映 Actor 启动耗时。
	ActorLaunchDurationHistogram = "actor_launch_duration_seconds"

	// ActorLifetimeHistogram Actor 从 Launched 到 Killed 的存活时长（秒）。
	ActorLifetimeHistogram = "actor_lifetime_seconds"

	// KilledActorTotalCounter 系统启动以来被 Kill 的 Actor 总次数。
	KilledActorTotalCounter = "killed_actor_total"

	// RestartedActorTotalCounter 进入重启流程的总次数（含 Restarting 事件）。
	RestartedActorTotalCounter = "restarted_actor_total"

	// ActorRestartSucceededTotalCounter 成功完成重启（Restarted）的次数。
	ActorRestartSucceededTotalCounter = "actor_restart_succeeded_total"

	// ActorFailedTotalCounter Actor 进入 Failed 状态的总次数（监督策略决定后的失败）。
	ActorFailedTotalCounter = "actor_failed_total"

	// ActorWatchTotalCounter Watch 调用总次数。
	ActorWatchTotalCounter = "actor_watch_total"

	// ActorWatchCountGauge 当前被 Watch 的 Actor 数量（Watch +1，Unwatch -1）。
	ActorWatchCountGauge = "actor_watch_count"

	// ActorUnwatchTotalCounter Unwatch 调用总次数。
	ActorUnwatchTotalCounter = "actor_unwatch_total"
)

// 邮箱暂停
const (
	// MailboxPausedTotalCounter 邮箱进入暂停状态的总次数。
	MailboxPausedTotalCounter = "mailbox_paused_total"

	// MailboxPausedCountGauge 当前处于暂停状态的邮箱数量（Paused +1，Resumed -1）。
	MailboxPausedCountGauge = "mailbox_paused_count"

	// MailboxPausedDurationSecondsHistogram 邮箱从 Paused 到 Resumed 的暂停时长（秒）。
	MailboxPausedDurationSecondsHistogram = "mailbox_paused_duration_seconds"

	// MailboxResumedTotalCounter 邮箱从暂停恢复的总次数。
	MailboxResumedTotalCounter = "mailbox_resumed_total"
)

// 网络
const (
	// RemotingInboundConnectionsTotalCounter 入站连接建立成功总次数。
	RemotingInboundConnectionsTotalCounter = "remoting_inbound_connections_total"

	// RemotingOutboundConnectionsTotalCounter 出站连接建立成功总次数。
	RemotingOutboundConnectionsTotalCounter = "remoting_outbound_connections_total"

	// RemotingConnectionFailedTotalCounter 连接失败总次数（Dial/握手/激活失败或重试用尽）。
	RemotingConnectionFailedTotalCounter = "remoting_connection_failed_total"

	// RemotingConnectionClosedTotalCounter 连接关闭总次数（读端异常、对端关闭、写/心跳失败等）。
	RemotingConnectionClosedTotalCounter = "remoting_connection_closed_total"

	// RemotingEnvelopSendFailedTotalCounter 向远程发送 Envelop 失败总次数。
	RemotingEnvelopSendFailedTotalCounter = "remoting_envelop_send_failed_total"

	// RemotingEnvelopSentTotalCounter 成功发送到远程的 Envelop 总条数（写完成并 ack）。
	RemotingEnvelopSentTotalCounter = "remoting_envelop_sent_total"

	// RemotingEnvelopReceivedTotalCounter 成功从远程接收并投递的 Envelop 总条数。
	RemotingEnvelopReceivedTotalCounter = "remoting_envelop_received_total"

	// RemotingBytesSentTotalCounter Remoting 发送的字节总数（含帧头），用于流量与速率统计。
	RemotingBytesSentTotalCounter = "remoting_bytes_sent_total"

	// RemotingBytesReceivedTotalCounter Remoting 接收的字节总数（含帧头），用于流量与速率统计。
	RemotingBytesReceivedTotalCounter = "remoting_bytes_received_total"

	// RemotingOutboundEndpointsGauge 当前出站 Endpoint 数量（每个远程地址一个 Endpoint，Launch +1、Kill -1）。
	RemotingOutboundEndpointsGauge = "remoting_outbound_endpoints"

	// RemotingPendingEnvelopsGauge 当前待发送的 Envelop 总数（各 Endpoint 出站队列之和）。
	RemotingPendingEnvelopsGauge = "remoting_pending_envelops"

	// RemotingEnvelopSizeBytesHistogram 单条 Envelop 载荷大小（字节）分布，用于观察消息体积。
	RemotingEnvelopSizeBytesHistogram = "remoting_envelop_size_bytes"
)
