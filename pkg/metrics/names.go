package metrics

// 以下常量为 Vivid 指标收集器使用的指标名称，供 internal/metrics、internal/cluster 等统一引用，
// 避免魔法字符串分散在代码中，便于控制台、监控系统按名称消费指标。

// ---- Actor 生命周期与数量 ----

// NameCounterActorSpawnedTotal 计数器：系统启动以来通过 ActorOf 创建的 Actor 总次数（每次创建 +1）。
const NameCounterActorSpawnedTotal = "vivid_actor_spawned_total"

// NameGaugeActorCount 仪表盘：当前存活的 Actor 数量（创建 +1，Kill -1）。
const NameGaugeActorCount = "vivid_actor_count"

// NameCounterActorSpawnedTotalByType 计数器：按类型细分的 Actor 创建次数（可扩展为按类型打点）。
const NameCounterActorSpawnedTotalByType = "vivid_actor_spawned_total_by_type"

// NameHistogramActorLaunchDurationSeconds 直方图：从 Spawned 到 Launched 的耗时（秒），反映 Actor 启动耗时。
const NameHistogramActorLaunchDurationSeconds = "vivid_actor_launch_duration_seconds"

// NameHistogramActorLifetimeSeconds 直方图：Actor 从 Launched 到 Killed 的存活时长（秒）。
const NameHistogramActorLifetimeSeconds = "vivid_actor_lifetime_seconds"

// NameCounterActorKilledTotal 计数器：系统启动以来被 Kill 的 Actor 总次数。
const NameCounterActorKilledTotal = "vivid_actor_killed_total"

// ---- Actor 重启 ----

// NameCounterActorRestartTotal 计数器：进入重启流程的总次数（含 Restarting 事件）。
const NameCounterActorRestartTotal = "vivid_actor_restart_total"

// NameCounterActorRestartTotalByType 计数器：按类型细分的重启次数。
const NameCounterActorRestartTotalByType = "vivid_actor_restart_total_by_type"

// NameHistogramActorRestartDurationSeconds 直方图：从 Restarting 到 Restarted 的重启耗时（秒）。
const NameHistogramActorRestartDurationSeconds = "vivid_actor_restart_duration_seconds"

// NameCounterActorRestartSuccessTotal 计数器：成功完成重启（Restarted）的次数。
const NameCounterActorRestartSuccessTotal = "vivid_actor_restart_success_total"

// ---- Actor 失败 ----

// NameCounterActorFailedTotal 计数器：Actor 进入 Failed 状态的总次数（监督策略决定后的失败）。
const NameCounterActorFailedTotal = "vivid_actor_failed_total"

// NameCounterActorFailedTotalByType 计数器：按类型细分的失败次数。
const NameCounterActorFailedTotalByType = "vivid_actor_failed_total_by_type"

// ---- Watch / Unwatch ----

// NameCounterActorWatchTotal 计数器：Watch 调用总次数。
const NameCounterActorWatchTotal = "vivid_actor_watch_total"

// NameGaugeActorWatchCount 仪表盘：当前被 Watch 的 Actor 数量（Watch +1，Unwatch -1）。
const NameGaugeActorWatchCount = "vivid_actor_watch_count"

// NameCounterActorUnwatchTotal 计数器：Unwatch 调用总次数。
const NameCounterActorUnwatchTotal = "vivid_actor_unwatch_total"

// ---- 邮箱暂停 / 恢复 ----

// NameCounterMailboxPausedTotal 计数器：邮箱进入暂停状态的总次数。
const NameCounterMailboxPausedTotal = "vivid_mailbox_paused_total"

// NameGaugeMailboxPausedCount 仪表盘：当前处于暂停状态的邮箱数量（Paused +1，Resumed -1）。
const NameGaugeMailboxPausedCount = "vivid_mailbox_paused_count"

// NameHistogramMailboxPausedDurationSeconds 直方图：邮箱从 Paused 到 Resumed 的暂停时长（秒）。
const NameHistogramMailboxPausedDurationSeconds = "vivid_mailbox_paused_duration_seconds"

// NameCounterMailboxResumedTotal 计数器：邮箱从暂停恢复的总次数。
const NameCounterMailboxResumedTotal = "vivid_mailbox_resumed_total"

// ---- 死信 ----

// NameCounterDeathLetterTotal 计数器：投递到死信队列的消息总数。
const NameCounterDeathLetterTotal = "vivid_death_letter_total"

// ---- 消息与事件 ----

// NameCounterMessagesProcessedTotal 计数器：已投递并进入 Actor OnReceive 的消息总数（不含系统消息如 OnLaunch/OnKill）。
const NameCounterMessagesProcessedTotal = "vivid_messages_processed_total"

// NameCounterStreamEventsTotal 计数器：通过 EventStream.Publish 发布的事件总数（生命周期、Watch、Mailbox 等）。
const NameCounterStreamEventsTotal = "vivid_stream_events_total"

// ---- 集群（cluster.*）----

// NameGaugeClusterMembers 仪表盘：当前视图中的集群成员总数。
const NameGaugeClusterMembers = "cluster.members"

// NameGaugeClusterHealthy 仪表盘：当前视图中状态为健康的成员数量。
const NameGaugeClusterHealthy = "cluster.healthy"

// NameGaugeClusterUnhealthy 仪表盘：当前视图中不健康的成员数量。
const NameGaugeClusterUnhealthy = "cluster.unhealthy"

// NameGaugeClusterQuorumSize 仪表盘：当前视图的法定人数（Quorum）大小。
const NameGaugeClusterQuorumSize = "cluster.quorum_size"

// NameGaugeClusterInQuorum 仪表盘：本节点是否在法定人数内（1=是，0=否）。
const NameGaugeClusterInQuorum = "cluster.in_quorum"

// NameGaugeClusterViewDivergence 仪表盘：本地视图与对比视图的成员差异数（用于检测视图分歧）。
const NameGaugeClusterViewDivergence = "cluster.view_divergence"

// NamePrefixClusterDC 集群按数据中心（DC）细分的指标名前缀，完整名为 NamePrefixClusterDC + dc + NameSuffixClusterDCMembers 或 NameSuffixClusterDCHealthy。
// 例如：cluster.dc._default.members、cluster.dc.AsiaChina.healthy。
const NamePrefixClusterDC = "cluster.dc."

// NameSuffixClusterDCMembers 数据中心维度成员数指标后缀，全名：NamePrefixClusterDC + dc + NameSuffixClusterDCMembers。
const NameSuffixClusterDCMembers = ".members"

// NameSuffixClusterDCHealthy 数据中心维度健康数指标后缀，全名：NamePrefixClusterDC + dc + NameSuffixClusterDCHealthy。
const NameSuffixClusterDCHealthy = ".healthy"
