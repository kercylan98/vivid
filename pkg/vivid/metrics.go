package vivid

import (
	"github.com/kercylan98/vivid/pkg/vivid/metrics"
	"time"
)

// 指标名称常量
const (
	// Actor相关指标
	MetricActorMailboxSize        = "actor_mailbox_size"
	MetricActorMailboxUtilization = "mailbox_utilization_percent"
	MetricAliveActorNum           = "alive_actor_num"
	MetricStoppingActorNum        = "stopping_actor_num"
	MetricRestartingActorNum      = "restarting_actor_num"
	MetricActorHierarchyDepth     = "actor_hierarchy_depth"
	MetricActorMemoryUsageBytes   = "actor_memory_usage_bytes"

	// 消息处理相关指标
	MetricProcessorMessageNum      = "processor_message_num"
	MetricMessageHandleDuration    = "message_handle_duration"
	MetricMessageHandledTotal      = "message_handled_total"
	MetricMessageQueuedTotal       = "message_queued_total"
	MetricMessageHandleErrorsTotal = "message_handle_errors_total"
	MetricMessageDroppedTotal      = "message_dropped_total"
	MetricMessageRetryTotal        = "message_retry_total"

	// Actor生命周期相关指标
	MetricActorLaunchedTotal     = "actor_launched_total"
	MetricActorKilledTotal       = "actor_killed_total"
	MetricActorRestartTotal      = "actor_restart_total"
	MetricActorPanicTotal        = "actor_panic_total"
	MetricActorTimeoutTotal      = "actor_timeout_total"
	MetricActorTimeoutDuration   = "actor_timeout_duration"
	MetricActorChildCreatedTotal = "actor_child_created_total"

	// 邮箱相关指标
	MetricMailboxOverflowTotal     = "mailbox_overflow_total"
	MetricMailboxFullWarningsTotal = "mailbox_full_warnings_total"
	MetricDeadLetterMessagesTotal  = "dead_letter_messages_total"

	// 系统监控相关指标
	MetricSystemCPUUsagePercent    = "system_cpu_usage_percent"
	MetricSystemMemoryUsagePercent = "system_memory_usage_percent"
	MetricSystemGoroutineNum       = "system_goroutine_num"
	MetricSystemGCPauseTime        = "system_gc_pause_time"

	// 监督相关指标
	MetricSupervisionStrategyAppliedTotal = "supervision_strategy_applied_total"

	// 网络相关指标
	MetricNetworkConnectionsTotal   = "network_connections_total"
	MetricNetworkBytesReceivedTotal = "network_bytes_received_total"
	MetricNetworkBytesSentTotal     = "network_bytes_sent_total"
)

// 标签键常量
const (
	TagKeyType       = "type"
	TagKeyRef        = "ref"
	TagKeyErrorType  = "error_type"
	TagKeyReason     = "reason"
	TagKeySupervisor = "supervisor"
	TagKeyChild      = "child"
	TagKeyStrategy   = "strategy"
	TagKeySender     = "sender"
	TagKeyReceiver   = "receiver"
	TagKeyParent     = "parent"
	TagKeyState      = "state"
	TagKeyProtocol   = "protocol"
	TagKeyDirection  = "directive"
)

// 标签值常量
const (
	TagValueSystem   = "system"
	TagValueUser     = "user"
	TagValueRestart  = "restart"
	TagValuePanic    = "panic"
	TagValueTimeout  = "timeout"
	TagValueInbound  = "inbound"
	TagValueOutbound = "outbound"
	TagValueTCP      = "tcp"
	TagValueUDP      = "udp"
	TagValueHTTP     = "http"
)

var (
	// 预定义的常用标签，避免重复创建
	tagTypeSystem = metrics.WithTag(TagKeyType, TagValueSystem)
	tagTypeUser   = metrics.WithTag(TagKeyType, TagValueUser)

	// 预定义的BucketProvider，避免重复创建
	durationBucketProvider = metrics.HistogramBucketProviderFN(func() []metrics.Bucket {
		return metrics.ExponentialBuckets(0.001, 2, 10) // 1ms到1s的指数分布桶
	})

	timeoutBucketProvider = metrics.HistogramBucketProviderFN(func() []metrics.Bucket {
		return metrics.ExponentialBuckets(0.001, 2, 15) // 1ms到32s的指数分布桶
	})

	sizeBucketProvider = metrics.HistogramBucketProviderFN(func() []metrics.Bucket {
		return metrics.ExponentialBuckets(1, 2, 20) // 1B到1MB的指数分布桶
	})

	gcPauseBucketProvider = metrics.HistogramBucketProviderFN(func() []metrics.Bucket {
		return metrics.ExponentialBuckets(0.0001, 2, 15) // 0.1ms到3.2s的指数分布桶
	})
)

type ActorSystemMetrics interface {
}

func newActorSystemMetrics(manager metrics.Manager) *actorSystemMetrics {
	return &actorSystemMetrics{
		Manager: manager,
	}
}

type actorSystemMetrics struct {
	HookCore
	metrics.Manager
}

func (m *actorSystemMetrics) OnActorHandleSystemMessageBefore(sender ActorRef, receiver ActorRef, message Message) {
	m.Gauge(MetricActorMailboxSize,
		tagTypeSystem,
		metrics.WithTag(TagKeyRef, receiver.GetPath()),
	).Dec()
	m.Gauge(MetricProcessorMessageNum, tagTypeSystem).Inc()
}

func (m *actorSystemMetrics) OnActorHandleUserMessageBefore(sender ActorRef, receiver ActorRef, message Message) {
	m.Gauge(MetricActorMailboxSize,
		tagTypeUser,
		metrics.WithTag(TagKeyRef, receiver.GetPath()),
	).Dec()
	m.Gauge(MetricProcessorMessageNum, tagTypeUser).Inc()
}

func (m *actorSystemMetrics) OnActorHandleSystemMessageAfter(sender ActorRef, receiver ActorRef, message Message, duration time.Duration) {
	refTag := metrics.WithTag(TagKeyRef, receiver.GetPath())
	m.Gauge(MetricProcessorMessageNum, tagTypeSystem).Dec()
	m.Histogram(MetricMessageHandleDuration,
		durationBucketProvider,
		tagTypeSystem,
		refTag,
	).Observe(duration.Seconds())
	m.Counter(MetricMessageHandledTotal,
		tagTypeSystem,
		refTag,
	).Inc()
}

func (m *actorSystemMetrics) OnActorHandleUserMessageAfter(sender ActorRef, receiver ActorRef, message Message, duration time.Duration) {
	refTag := metrics.WithTag(TagKeyRef, receiver.GetPath())
	m.Gauge(MetricProcessorMessageNum, tagTypeUser).Dec()
	m.Histogram(MetricMessageHandleDuration,
		durationBucketProvider,
		tagTypeUser,
		refTag,
	).Observe(duration.Seconds())
	m.Counter(MetricMessageHandledTotal,
		tagTypeUser,
		refTag,
	).Inc()
}

func (m *actorSystemMetrics) OnActorHandleMessageError(sender ActorRef, receiver ActorRef, message Message, err error) {
	m.Counter(MetricMessageHandleErrorsTotal,
		metrics.WithTag(TagKeyRef, receiver.GetPath()),
		metrics.WithTag(TagKeyErrorType, err.Error()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorMailboxPushUserMessageBefore(ref ActorRef, message Message) {
	m.Gauge(MetricActorMailboxSize,
		tagTypeUser,
		metrics.WithTag(TagKeyRef, ref.GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorMailboxPushSystemMessageBefore(ref ActorRef, message Message) {
	m.Gauge(MetricActorMailboxSize,
		tagTypeSystem,
		metrics.WithTag(TagKeyRef, ref.GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorMailboxPushUserMessageAfter(ref ActorRef, message Message) {
	m.Counter(MetricMessageQueuedTotal,
		tagTypeUser,
		metrics.WithTag(TagKeyRef, ref.GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorMailboxPushSystemMessageAfter(ref ActorRef, message Message) {
	m.Counter(MetricMessageQueuedTotal,
		tagTypeSystem,
		metrics.WithTag(TagKeyRef, ref.GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorMailboxOverflow(ref ActorRef, message Message) {
	m.Counter(MetricMailboxOverflowTotal,
		metrics.WithTag(TagKeyRef, ref.GetPath()),
	).Inc()
	m.Counter(MetricMessageDroppedTotal,
		metrics.WithTag(TagKeyRef, ref.GetPath()),
		metrics.WithTag(TagKeyReason, "mailbox_overflow"),
	).Inc()
}

func (m *actorSystemMetrics) OnActorLaunch(ctx ActorContext) {
	// 当前存活的 Actor 数量
	m.Gauge(MetricAliveActorNum).Inc()
	m.Counter(MetricActorLaunchedTotal,
		metrics.WithTag(TagKeyRef, ctx.Ref().GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorRestart(ctx ActorContext, reason interface{}) {
	refTag := metrics.WithTag(TagKeyRef, ctx.Ref().GetPath())
	m.Counter(MetricActorRestartTotal,
		refTag,
		metrics.WithTag(TagKeyReason, TagValueRestart),
	).Inc()
	m.Gauge(MetricRestartingActorNum).Inc()
}

func (m *actorSystemMetrics) OnActorRestarted(ctx ActorContext) {
	m.Gauge(MetricRestartingActorNum).Dec()
}

func (m *actorSystemMetrics) OnActorKill(ctx ActorContext, message *OnKill) {
	// 当前正在停止中的 Actor 数量
	m.Gauge(MetricStoppingActorNum).Inc()
}

func (m *actorSystemMetrics) OnActorKilled(message *OnKilled) {
	// 当前存活的 Actor 数量
	m.Gauge(MetricAliveActorNum).Dec()
	m.Gauge(MetricStoppingActorNum).Dec()
	m.Counter(MetricActorKilledTotal,
		metrics.WithTag(TagKeyRef, message.Ref().GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorPanic(ctx ActorContext, reason interface{}) {
	m.Counter(MetricActorPanicTotal,
		metrics.WithTag(TagKeyRef, ctx.Ref().GetPath()),
		metrics.WithTag(TagKeyReason, TagValuePanic),
	).Inc()
}

func (m *actorSystemMetrics) OnActorTimeout(ref ActorRef, duration time.Duration) {
	refTag := metrics.WithTag(TagKeyRef, ref.GetPath())
	m.Counter(MetricActorTimeoutTotal, refTag).Inc()
	m.Histogram(MetricActorTimeoutDuration,
		timeoutBucketProvider,
		refTag,
	).Observe(duration.Seconds())
}

func (m *actorSystemMetrics) OnActorMemoryUsage(ref ActorRef, memoryBytes uint64) {
	m.Gauge(MetricActorMemoryUsageBytes,
		metrics.WithTag(TagKeyRef, ref.GetPath()),
	).Set(float64(memoryBytes))
}

func (m *actorSystemMetrics) OnSystemLoad(cpuPercent float64, memoryPercent float64) {
	m.Gauge(MetricSystemCPUUsagePercent).Set(cpuPercent)
	m.Gauge(MetricSystemMemoryUsagePercent).Set(memoryPercent)
}

func (m *actorSystemMetrics) OnActorSupervisionStrategyApplied(supervisor ActorRef, child ActorRef, strategy string) {
	m.Counter(MetricSupervisionStrategyAppliedTotal,
		metrics.WithTag(TagKeySupervisor, supervisor.GetPath()),
		metrics.WithTag(TagKeyChild, child.GetPath()),
		metrics.WithTag(TagKeyStrategy, strategy),
	).Inc()
}

func (m *actorSystemMetrics) OnDeadLetterMessage(sender ActorRef, receiver ActorRef, message Message) {
	m.Counter(MetricDeadLetterMessagesTotal,
		metrics.WithTag(TagKeySender, sender.GetPath()),
		metrics.WithTag(TagKeyReceiver, receiver.GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorMailboxFullWarning(ref ActorRef, currentSize int, maxSize int) {
	refTag := metrics.WithTag(TagKeyRef, ref.GetPath())
	m.Counter(MetricMailboxFullWarningsTotal, refTag).Inc()
	m.Gauge(MetricActorMailboxUtilization,
		refTag,
	).Set(float64(currentSize) / float64(maxSize) * 100)
}

func (m *actorSystemMetrics) OnActorChildCreated(parent ActorRef, child ActorRef) {
	m.Counter(MetricActorChildCreatedTotal,
		metrics.WithTag(TagKeyParent, parent.GetPath()),
		metrics.WithTag(TagKeyChild, child.GetPath()),
	).Inc()
	// 计算Actor层级深度（基于路径中的'/'数量）
	pathParts := len(child.GetPath()) // 简化实现，实际应该计算路径分隔符
	m.Gauge(MetricActorHierarchyDepth,
		metrics.WithTag(TagKeyRef, child.GetPath()),
	).Set(float64(pathParts))
}

// 新增的高级指标方法

func (m *actorSystemMetrics) OnActorWatch(watcher ActorRef, watched ActorRef) {
	m.Counter("actor_watch_total",
		metrics.WithTag("watcher", watcher.GetPath()),
		metrics.WithTag("watched", watched.GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorUnwatch(watcher ActorRef, watched ActorRef) {
	m.Counter("actor_unwatch_total",
		metrics.WithTag("watcher", watcher.GetPath()),
		metrics.WithTag("watched", watched.GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorTerminate(ctx ActorContext, reason interface{}) {
	m.Counter("actor_terminate_total",
		metrics.WithTag(TagKeyRef, ctx.Ref().GetPath()),
		metrics.WithTag(TagKeyReason, "terminate"),
	).Inc()
}

func (m *actorSystemMetrics) OnActorReceiveTimeout(ctx ActorContext, duration time.Duration) {
	m.Counter("actor_receive_timeout_total",
		metrics.WithTag(TagKeyRef, ctx.Ref().GetPath()),
	).Inc()
	m.Histogram("actor_receive_timeout_duration",
		timeoutBucketProvider,
		metrics.WithTag(TagKeyRef, ctx.Ref().GetPath()),
	).Observe(duration.Seconds())
}

func (m *actorSystemMetrics) OnActorStateChange(ctx ActorContext, oldState string, newState string) {
	m.Counter("actor_state_change_total",
		metrics.WithTag(TagKeyRef, ctx.Ref().GetPath()),
		metrics.WithTag("old_state", oldState),
		metrics.WithTag("new_state", newState),
	).Inc()
}

func (m *actorSystemMetrics) OnSystemGoroutineCount(count int) {
	m.Gauge(MetricSystemGoroutineNum).Set(float64(count))
}

func (m *actorSystemMetrics) OnSystemGCPause(duration time.Duration) {
	m.Histogram(MetricSystemGCPauseTime,
		gcPauseBucketProvider,
	).Observe(duration.Seconds())
}

func (m *actorSystemMetrics) OnNetworkConnection(protocol string, direction string) {
	m.Counter(MetricNetworkConnectionsTotal,
		metrics.WithTag(TagKeyProtocol, protocol),
		metrics.WithTag(TagKeyDirection, direction),
	).Inc()
}

func (m *actorSystemMetrics) OnNetworkBytesTransferred(direction string, bytes uint64) {
	if direction == TagValueInbound {
		m.Counter(MetricNetworkBytesReceivedTotal).Add(bytes)
	} else {
		m.Counter(MetricNetworkBytesSentTotal).Add(bytes)
	}
}

func (m *actorSystemMetrics) OnMessageRetry(ref ActorRef, attempt int, reason string) {
	m.Counter(MetricMessageRetryTotal,
		metrics.WithTag(TagKeyRef, ref.GetPath()),
		metrics.WithTag(TagKeyReason, reason),
	).Inc()
	m.Histogram("message_retry_attempt",
		metrics.HistogramBucketProviderFN(func() []metrics.Bucket {
			return metrics.LinearBuckets(1, 1, 10) // 1-10次重试的线性分布桶
		}),
		metrics.WithTag(TagKeyRef, ref.GetPath()),
	).Observe(float64(attempt))
}

func (m *actorSystemMetrics) hooks() []Hook {
	return []Hook{m}
}
