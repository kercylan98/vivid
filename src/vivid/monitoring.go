// Package vivid 提供了一个高性能的 Actor 系统实现，包含监控和指标收集功能。
package vivid

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricsSnapshot 包含某个时间点的所有指标数据快照。
type MetricsSnapshot struct {
	Timestamp time.Time `json:"timestamp"`

	// 消息指标
	MessagesSent       int64   `json:"messages_sent"`
	MessagesReceived   int64   `json:"messages_received"`
	MessagesDeadLetter int64   `json:"messages_dead_letter"`
	MessageThroughput  float64 `json:"message_throughput"` // 每秒消息数

	// 分类消息指标
	UserMessagesSent       int64 `json:"user_messages_sent"`       // 用户消息发送数
	UserMessagesReceived   int64 `json:"user_messages_received"`   // 用户消息接收数
	SystemMessagesSent     int64 `json:"system_messages_sent"`     // 系统消息发送数
	SystemMessagesReceived int64 `json:"system_messages_received"` // 系统消息接收数

	// 延迟指标
	AverageLatency time.Duration `json:"average_latency"`
	MaxLatency     time.Duration `json:"max_latency"`
	MinLatency     time.Duration `json:"min_latency"`
	P95Latency     time.Duration `json:"p95_latency"`
	P99Latency     time.Duration `json:"p99_latency"`

	// Actor指标
	ActiveActors     int64 `json:"active_actors"`
	CreatedActors    int64 `json:"created_actors"`
	TerminatedActors int64 `json:"terminated_actors"`
	RestartedActors  int64 `json:"restarted_actors"`

	// 错误指标
	ErrorCount int64   `json:"error_count"`
	ErrorRate  float64 `json:"error_rate"` // 错误率百分比

	// 系统指标
	MemoryUsage    int64 `json:"memory_usage"`    // 内存使用量（字节）
	GoroutineCount int   `json:"goroutine_count"` // 协程数量

	// 队列指标
	MailboxDepth int64 `json:"mailbox_depth"` // 平均邮箱深度

	// 运行时间
	UptimeDuration time.Duration `json:"uptime_duration"`
}

// ActorMetrics 包含 Actor 级别的指标数据。
type ActorMetrics struct {
	ActorRef        ActorRef      `json:"actor_ref"`
	ActorName       string        `json:"actor_name"`
	MessagesHandled int64         `json:"messages_handled"`
	LastMessageTime time.Time     `json:"last_message_time"`
	AverageLatency  time.Duration `json:"average_latency"`
	ErrorCount      int64         `json:"error_count"`
	RestartCount    int64         `json:"restart_count"`
	State           string        `json:"state"` // "active", "terminated", "restarting"
}

// Metrics 定义了业务指标收集接口，仅包含用户级功能。
type Metrics interface {
	// IncrementCounter 增加指定名称计数器的值
	IncrementCounter(name string, value int64, tags ...string)

	// DecrementCounter 减少指定名称计数器的值
	DecrementCounter(name string, value int64, tags ...string)

	// GetCounter 获取指定名称计数器的当前值
	GetCounter(name string) int64

	// SetGauge 设置指定名称计量器的值
	SetGauge(name string, value float64, tags ...string)

	// GetGauge 获取指定名称计量器的当前值
	GetGauge(name string) float64

	// RecordHistogram 记录直方图数据点
	RecordHistogram(name string, value float64, tags ...string)

	// GetHistogram 获取指定名称直方图的数据统计
	GetHistogram(name string) *HistogramData

	// RecordTiming 记录时间测量数据
	RecordTiming(name string, duration time.Duration, tags ...string)

	// GetTiming 获取指定名称时间测量的统计数据
	GetTiming(name string) *TimingData

	// RecordError 记录业务错误信息
	RecordError(actor ActorRef, err error, context string)

	// GetSystemSnapshot 获取系统指标的只读快照
	GetSystemSnapshot() SystemMetricsSnapshot

	// GetCustomMetricsSnapshot 获取自定义指标的快照
	GetCustomMetricsSnapshot() CustomMetricsSnapshot

	// GetActorMetrics 获取指定 Actor 的指标数据
	GetActorMetrics(actor ActorRef) *ActorMetrics

	// GetAllActorMetrics 获取所有 Actor 的指标数据
	GetAllActorMetrics() []ActorMetrics

	// Reset 重置所有指标数据
	Reset()

	// IsEnabled 检查指标收集是否启用
	IsEnabled() bool
}

// SystemMetricsSnapshot 包含系统指标的只读快照数据。
type SystemMetricsSnapshot struct {
	Timestamp time.Time `json:"timestamp"`

	// 系统消息指标（只读）
	MessagesSent       int64   `json:"messages_sent"`
	MessagesReceived   int64   `json:"messages_received"`
	MessagesDeadLetter int64   `json:"messages_dead_letter"`
	MessageThroughput  float64 `json:"message_throughput"`

	// 延迟指标
	AverageLatency time.Duration `json:"average_latency"`
	MaxLatency     time.Duration `json:"max_latency"`
	MinLatency     time.Duration `json:"min_latency"`

	// Actor指标
	ActiveActors     int64 `json:"active_actors"`
	CreatedActors    int64 `json:"created_actors"`
	TerminatedActors int64 `json:"terminated_actors"`
	RestartedActors  int64 `json:"restarted_actors"`

	// 运行时间
	UptimeDuration time.Duration `json:"uptime_duration"`
}

// CustomMetricsSnapshot 包含自定义指标的快照数据。
type CustomMetricsSnapshot struct {
	Timestamp  time.Time                 `json:"timestamp"`
	Counters   map[string]int64          `json:"counters"`
	Gauges     map[string]float64        `json:"gauges"`
	Histograms map[string]*HistogramData `json:"histograms"`
	Timings    map[string]*TimingData    `json:"timings"`
}

// HistogramData 包含直方图的统计数据。
type HistogramData struct {
	Count  int64   `json:"count"`
	Sum    float64 `json:"sum"`
	Mean   float64 `json:"mean"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Median float64 `json:"median"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
}

// TimingData 包含时间测量的统计数据。
type TimingData struct {
	Count  int64         `json:"count"`
	Sum    time.Duration `json:"sum"`
	Mean   time.Duration `json:"mean"`
	Min    time.Duration `json:"min"`
	Max    time.Duration `json:"max"`
	Median time.Duration `json:"median"`
	P95    time.Duration `json:"p95"`
	P99    time.Duration `json:"p99"`
}

// DeadLetterMessage 表示一个死信消息的结构。
type DeadLetterMessage struct {
	From      ActorRef  `json:"from"`
	To        ActorRef  `json:"to"`
	Message   Message   `json:"message"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
	Attempts  int       `json:"attempts"` // 重试次数
}

// DeadLetterHandler 定义了死信处理器的接口。
type DeadLetterHandler interface {
	// HandleDeadLetter 处理单个死信消息
	HandleDeadLetter(deadLetter DeadLetterMessage)

	// GetDeadLetters 获取所有死信消息的列表
	GetDeadLetters() []DeadLetterMessage

	// GetDeadLetterCount 获取死信消息的总数量
	GetDeadLetterCount() int64

	// ClearDeadLetters 清空所有死信消息
	ClearDeadLetters()
}

// MonitoringConfig 包含监控系统的配置选项。
type MonitoringConfig struct {
	// EnableMetrics 指示是否启用指标收集
	EnableMetrics bool

	// EnableDeadLetterHandling 指示是否启用死信处理
	EnableDeadLetterHandling bool

	// MetricsInterval 指定指标收集的时间间隔
	MetricsInterval time.Duration

	// DeadLetterQueueSize 指定死信队列的最大大小
	DeadLetterQueueSize int

	// MaxActorMetrics 指定最大 Actor 指标数量
	MaxActorMetrics int

	// CustomMetricsExporter 用于导出自定义指标的回调函数
	CustomMetricsExporter func(system SystemMetricsSnapshot, custom CustomMetricsSnapshot)

	// CustomDeadLetterHandler 自定义的死信处理器
	CustomDeadLetterHandler DeadLetterHandler
}

// DefaultMonitoringConfig 返回具有默认值的监控配置。
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		EnableMetrics:            true,
		EnableDeadLetterHandling: true,
		MetricsInterval:          time.Second * 10,
		DeadLetterQueueSize:      1000,
		MaxActorMetrics:          10000,
	}
}

// WithEnableMetrics 设置是否启用指标收集并返回配置实例。
func (c *MonitoringConfig) WithEnableMetrics(enable bool) *MonitoringConfig {
	c.EnableMetrics = enable
	return c
}

// WithEnableDeadLetterHandling 设置是否启用死信处理并返回配置实例。
func (c *MonitoringConfig) WithEnableDeadLetterHandling(enable bool) *MonitoringConfig {
	c.EnableDeadLetterHandling = enable
	return c
}

// WithMetricsInterval 设置指标收集间隔并返回配置实例。
func (c *MonitoringConfig) WithMetricsInterval(interval time.Duration) *MonitoringConfig {
	c.MetricsInterval = interval
	return c
}

// WithDeadLetterQueueSize 设置死信队列大小并返回配置实例。
func (c *MonitoringConfig) WithDeadLetterQueueSize(size int) *MonitoringConfig {
	c.DeadLetterQueueSize = size
	return c
}

// WithMaxActorMetrics 设置最大 Actor 指标数量并返回配置实例。
func (c *MonitoringConfig) WithMaxActorMetrics(max int) *MonitoringConfig {
	c.MaxActorMetrics = max
	return c
}

// WithCustomMetricsExporter 设置自定义指标导出器并返回配置实例。
func (c *MonitoringConfig) WithCustomMetricsExporter(exporter func(system SystemMetricsSnapshot, custom CustomMetricsSnapshot)) *MonitoringConfig {
	c.CustomMetricsExporter = exporter
	return c
}

// WithCustomDeadLetterHandler 设置自定义死信处理器并返回配置实例。
func (c *MonitoringConfig) WithCustomDeadLetterHandler(handler DeadLetterHandler) *MonitoringConfig {
	c.CustomDeadLetterHandler = handler
	return c
}

// metricsCollector 是指标收集器的默认实现。
type metricsCollector struct {
	// 系统指标（内部使用）
	messagesSent     int64
	messagesReceived int64
	deadLetterCount  int64
	activeActors     int64
	createdActors    int64
	terminatedActors int64
	restartedActors  int64
	errorCount       int64

	// 分类消息计数器（内部使用）
	userMessagesSent       int64
	userMessagesReceived   int64
	systemMessagesSent     int64
	systemMessagesReceived int64

	// 延迟统计（内部使用）
	latencySum   int64 // 纳秒
	latencyCount int64
	maxLatency   int64 // 纳秒
	minLatency   int64 // 纳秒

	// 自定义指标（用户使用）
	customCounters   map[string]int64
	customGauges     map[string]float64
	customHistograms map[string]*HistogramData
	customTimings    map[string]*TimingData
	customMu         sync.RWMutex

	// Actor指标映射
	actorMetricsMu sync.RWMutex
	actorMetrics   map[string]*ActorMetrics

	// 配置
	config *MonitoringConfig

	// 启动时间
	startTime time.Time

	// 记录状态
	recording int64 // 原子操作，1表示正在记录，0表示停止记录
}

// NewMetricsCollector 创建一个新的指标收集器
func NewMetricsCollector(config *MonitoringConfig) Metrics {
	if config == nil {
		config = DefaultMonitoringConfig()
	}

	collector := &metricsCollector{
		config:           config,
		recording:        1,
		startTime:        time.Now(),
		minLatency:       int64(time.Hour),
		actorMetrics:     make(map[string]*ActorMetrics),
		customCounters:   make(map[string]int64),
		customGauges:     make(map[string]float64),
		customHistograms: make(map[string]*HistogramData),
		customTimings:    make(map[string]*TimingData),
	}

	return collector
}

// NewSimpleMetrics 创建一个简单的监控实例（仅启用基本指标收集）
func NewSimpleMetrics() Metrics {
	config := DefaultMonitoringConfig().
		WithEnableMetrics(true).
		WithEnableDeadLetterHandling(false).
		WithMetricsInterval(5 * time.Second)
	return NewMetricsCollector(config)
}

// NewProductionMetrics 创建一个生产环境推荐的监控实例（完整功能）
func NewProductionMetrics() Metrics {
	config := DefaultMonitoringConfig().
		WithEnableMetrics(true).
		WithEnableDeadLetterHandling(true).
		WithMetricsInterval(10 * time.Second).
		WithDeadLetterQueueSize(10000).
		WithMaxActorMetrics(1000)
	return NewMetricsCollector(config)
}

// NewDevelopmentMetrics 创建一个开发环境推荐的监控实例（详细调试信息）
func NewDevelopmentMetrics() Metrics {
	config := DefaultMonitoringConfig().
		WithEnableMetrics(true).
		WithEnableDeadLetterHandling(true).
		WithMetricsInterval(1 * time.Second).
		WithDeadLetterQueueSize(1000).
		WithMaxActorMetrics(100)
	return NewMetricsCollector(config)
}

// === 用户接口实现 ===

// IncrementCounter 增加计数器
func (m *metricsCollector) IncrementCounter(name string, value int64, tags ...string) {
	if !m.IsEnabled() {
		return
	}
	m.customMu.Lock()
	m.customCounters[name] += value
	m.customMu.Unlock()
}

// DecrementCounter 减少计数器
func (m *metricsCollector) DecrementCounter(name string, value int64, tags ...string) {
	if !m.IsEnabled() {
		return
	}
	m.customMu.Lock()
	m.customCounters[name] -= value
	m.customMu.Unlock()
}

// GetCounter 获取计数器值
func (m *metricsCollector) GetCounter(name string) int64 {
	m.customMu.RLock()
	defer m.customMu.RUnlock()
	return m.customCounters[name]
}

// SetGauge 设置计量器值
func (m *metricsCollector) SetGauge(name string, value float64, tags ...string) {
	if !m.IsEnabled() {
		return
	}
	m.customMu.Lock()
	m.customGauges[name] = value
	m.customMu.Unlock()
}

// GetGauge 获取计量器值
func (m *metricsCollector) GetGauge(name string) float64 {
	m.customMu.RLock()
	defer m.customMu.RUnlock()
	return m.customGauges[name]
}

// RecordHistogram 记录直方图数据
func (m *metricsCollector) RecordHistogram(name string, value float64, tags ...string) {
	if !m.IsEnabled() {
		return
	}
	m.customMu.Lock()
	defer m.customMu.Unlock()

	hist, exists := m.customHistograms[name]
	if !exists {
		hist = &HistogramData{Min: value, Max: value}
		m.customHistograms[name] = hist
	}

	hist.Count++
	hist.Sum += value
	hist.Mean = hist.Sum / float64(hist.Count)
	if value < hist.Min {
		hist.Min = value
	}
	if value > hist.Max {
		hist.Max = value
	}
}

// GetHistogram 获取直方图数据
func (m *metricsCollector) GetHistogram(name string) *HistogramData {
	m.customMu.RLock()
	defer m.customMu.RUnlock()
	if hist, exists := m.customHistograms[name]; exists {
		// 返回副本
		copy := *hist
		return &copy
	}
	return nil
}

// RecordTiming 记录时间数据
func (m *metricsCollector) RecordTiming(name string, duration time.Duration, tags ...string) {
	if !m.IsEnabled() {
		return
	}
	m.customMu.Lock()
	defer m.customMu.Unlock()

	timing, exists := m.customTimings[name]
	if !exists {
		timing = &TimingData{Min: duration, Max: duration}
		m.customTimings[name] = timing
	}

	timing.Count++
	timing.Sum += duration
	timing.Mean = time.Duration(int64(timing.Sum) / timing.Count)
	if duration < timing.Min {
		timing.Min = duration
	}
	if duration > timing.Max {
		timing.Max = duration
	}
}

// GetTiming 获取时间数据
func (m *metricsCollector) GetTiming(name string) *TimingData {
	m.customMu.RLock()
	defer m.customMu.RUnlock()
	if timing, exists := m.customTimings[name]; exists {
		// 返回副本
		copy := *timing
		return &copy
	}
	return nil
}

// GetSystemSnapshot 获取系统指标的只读快照
func (m *metricsCollector) GetSystemSnapshot() SystemMetricsSnapshot {
	return m.getSystemSnapshot()
}

// IsEnabled 检查指标收集是否启用
func (m *metricsCollector) IsEnabled() bool {
	return m.config.EnableMetrics && m.IsRecording()
}

// 确保metricsCollector实现了internalMetrics接口
var _ internalMetrics = (*metricsCollector)(nil)

// RecordMessageSent 记录消息发送（内部接口实现，默认为用户消息）
func (m *metricsCollector) RecordMessageSent(from, to ActorRef, messageType string) {
	m.recordUserMessageSent(from, to, messageType)
}

// RecordMessageReceived 记录消息接收（内部接口实现，默认为用户消息）
func (m *metricsCollector) RecordMessageReceived(actor ActorRef, messageType string, latency time.Duration) {
	m.recordUserMessageReceived(actor, messageType, latency)
}

// === 内部接口实现 ===

// getSystemSnapshot 内部方法：获取系统指标快照
func (m *metricsCollector) getSystemSnapshot() SystemMetricsSnapshot {
	now := time.Now()
	uptime := now.Sub(m.startTime)
	duration := uptime.Seconds()

	messagesSent := atomic.LoadInt64(&m.messagesSent)
	messagesReceived := atomic.LoadInt64(&m.messagesReceived)
	latencySum := atomic.LoadInt64(&m.latencySum)
	latencyCount := atomic.LoadInt64(&m.latencyCount)

	var avgLatency time.Duration
	if latencyCount > 0 {
		avgLatency = time.Duration(latencySum / latencyCount)
	}

	var throughput float64
	if duration > 0 {
		throughput = float64(messagesReceived) / duration
	}

	return SystemMetricsSnapshot{
		Timestamp:          now,
		MessagesSent:       messagesSent,
		MessagesReceived:   messagesReceived,
		MessagesDeadLetter: atomic.LoadInt64(&m.deadLetterCount),
		MessageThroughput:  throughput,
		AverageLatency:     avgLatency,
		MaxLatency:         time.Duration(atomic.LoadInt64(&m.maxLatency)),
		MinLatency:         time.Duration(atomic.LoadInt64(&m.minLatency)),
		ActiveActors:       atomic.LoadInt64(&m.activeActors),
		CreatedActors:      atomic.LoadInt64(&m.createdActors),
		TerminatedActors:   atomic.LoadInt64(&m.terminatedActors),
		RestartedActors:    atomic.LoadInt64(&m.restartedActors),
		UptimeDuration:     uptime,
	}
}

// recordUserMessageSent 内部方法：记录用户消息发送
func (m *metricsCollector) recordUserMessageSent(from, to ActorRef, messageType string) {
	if !m.isRecording() {
		return
	}
	atomic.AddInt64(&m.messagesSent, 1)
	atomic.AddInt64(&m.userMessagesSent, 1)
}

// recordSystemMessageSent 内部方法：记录系统消息发送
func (m *metricsCollector) recordSystemMessageSent(from, to ActorRef, messageType string) {
	if !m.isRecording() {
		return
	}
	atomic.AddInt64(&m.messagesSent, 1)
	atomic.AddInt64(&m.systemMessagesSent, 1)
}

// recordUserMessageReceived 内部方法：记录用户消息接收
func (m *metricsCollector) recordUserMessageReceived(actor ActorRef, messageType string, latency time.Duration) {
	if !m.isRecording() {
		return
	}
	atomic.AddInt64(&m.messagesReceived, 1)
	atomic.AddInt64(&m.userMessagesReceived, 1)
	m.updateLatencyStats(latency)
	m.updateActorMetrics(actor, messageType, latency, false)
}

// recordSystemMessageReceived 内部方法：记录系统消息接收
func (m *metricsCollector) recordSystemMessageReceived(actor ActorRef, messageType string, latency time.Duration) {
	if !m.isRecording() {
		return
	}
	atomic.AddInt64(&m.messagesReceived, 1)
	atomic.AddInt64(&m.systemMessagesReceived, 1)
	m.updateLatencyStats(latency)
	m.updateActorMetrics(actor, messageType, latency, false)
}

// recordMessageDeadLetter 内部方法：记录死信消息
func (m *metricsCollector) recordMessageDeadLetter(from, to ActorRef, message Message, reason string) {
	atomic.AddInt64(&m.deadLetterCount, 1)
}

// recordActorCreated 内部方法：记录Actor创建
func (m *metricsCollector) recordActorCreated(actor ActorRef, actorType string) {
	atomic.AddInt64(&m.createdActors, 1)
	atomic.AddInt64(&m.activeActors, 1)

	m.actorMetricsMu.Lock()
	defer m.actorMetricsMu.Unlock()

	key := m.getActorKey(actor)
	m.actorMetrics[key] = &ActorMetrics{
		ActorRef:        actor,
		ActorName:       actorType,
		MessagesHandled: 0,
		LastMessageTime: time.Now(),
		ErrorCount:      0,
		RestartCount:    0,
		State:           "active",
	}
}

// recordActorTerminated 内部方法：记录Actor终止
func (m *metricsCollector) recordActorTerminated(actor ActorRef, reason string) {
	atomic.AddInt64(&m.terminatedActors, 1)
	atomic.AddInt64(&m.activeActors, -1)

	m.actorMetricsMu.Lock()
	defer m.actorMetricsMu.Unlock()

	key := m.getActorKey(actor)
	if metrics, exists := m.actorMetrics[key]; exists {
		metrics.State = "terminated"
	}
}

// recordActorRestarted 内部方法：记录Actor重启
func (m *metricsCollector) recordActorRestarted(actor ActorRef, reason string) {
	atomic.AddInt64(&m.restartedActors, 1)
	m.updateActorMetrics(actor, "restart", 0, false)
}

// stopRecording 内部方法：停止记录
func (m *metricsCollector) stopRecording() {
	atomic.StoreInt64(&m.recording, 0)
}

// StopRecording 导出方法：停止记录
func (m *metricsCollector) StopRecording() {
	m.stopRecording()
}

// resumeRecording 内部方法：恢复记录
func (m *metricsCollector) resumeRecording() {
	atomic.StoreInt64(&m.recording, 1)
}

// ResumeRecording 导出方法：恢复记录
func (m *metricsCollector) ResumeRecording() {
	m.resumeRecording()
}

// isRecording 内部方法：检查是否正在记录
func (m *metricsCollector) isRecording() bool {
	return atomic.LoadInt64(&m.recording) == 1
}

// IsRecording 导出方法：检查是否正在记录
func (m *metricsCollector) IsRecording() bool {
	return m.isRecording()
}

func (m *metricsCollector) updateLatencyStats(latency time.Duration) {
	latencyNs := latency.Nanoseconds()
	atomic.AddInt64(&m.latencySum, latencyNs)
	atomic.AddInt64(&m.latencyCount, 1)

	// 更新最大延迟
	for {
		current := atomic.LoadInt64(&m.maxLatency)
		if latencyNs <= current || atomic.CompareAndSwapInt64(&m.maxLatency, current, latencyNs) {
			break
		}
	}

	// 更新最小延迟
	for {
		current := atomic.LoadInt64(&m.minLatency)
		if latencyNs >= current || atomic.CompareAndSwapInt64(&m.minLatency, current, latencyNs) {
			break
		}
	}
}

func (m *metricsCollector) RecordError(actor ActorRef, err error, context string) {
	atomic.AddInt64(&m.errorCount, 1)
	m.updateActorMetrics(actor, "error", 0, true)
}

func (m *metricsCollector) GetActorMetrics(actor ActorRef) *ActorMetrics {
	m.actorMetricsMu.RLock()
	defer m.actorMetricsMu.RUnlock()

	key := m.getActorKey(actor)
	if metrics, exists := m.actorMetrics[key]; exists {
		// 返回副本避免并发修改
		copy := *metrics
		return &copy
	}
	return nil
}

func (m *metricsCollector) GetAllActorMetrics() []ActorMetrics {
	m.actorMetricsMu.RLock()
	defer m.actorMetricsMu.RUnlock()

	result := make([]ActorMetrics, 0, len(m.actorMetrics))
	for _, metrics := range m.actorMetrics {
		result = append(result, *metrics)
	}
	return result
}

func (m *metricsCollector) Reset() {
	atomic.StoreInt64(&m.messagesSent, 0)
	atomic.StoreInt64(&m.messagesReceived, 0)
	atomic.StoreInt64(&m.deadLetterCount, 0)
	atomic.StoreInt64(&m.createdActors, 0)
	atomic.StoreInt64(&m.terminatedActors, 0)
	atomic.StoreInt64(&m.restartedActors, 0)
	atomic.StoreInt64(&m.errorCount, 0)
	atomic.StoreInt64(&m.userMessagesSent, 0)
	atomic.StoreInt64(&m.userMessagesReceived, 0)
	atomic.StoreInt64(&m.systemMessagesSent, 0)
	atomic.StoreInt64(&m.systemMessagesReceived, 0)
	atomic.StoreInt64(&m.latencySum, 0)
	atomic.StoreInt64(&m.latencyCount, 0)
	atomic.StoreInt64(&m.maxLatency, 0)
	atomic.StoreInt64(&m.minLatency, int64(time.Hour))

	m.actorMetricsMu.Lock()
	m.actorMetrics = make(map[string]*ActorMetrics)
	m.actorMetricsMu.Unlock()

	m.startTime = time.Now()
}

func (m *metricsCollector) updateActorMetrics(actor ActorRef, messageType string, latency time.Duration, isError bool) {
	// 如果actor为nil，直接返回
	if actor == nil {
		return
	}

	m.actorMetricsMu.Lock()
	defer m.actorMetricsMu.Unlock()

	key := m.getActorKey(actor)
	metrics, exists := m.actorMetrics[key]
	if !exists {
		return // Actor不存在
	}

	metrics.LastMessageTime = time.Now()
	if messageType == "restart" {
		metrics.RestartCount++
		metrics.State = "restarting"
	} else if isError {
		metrics.ErrorCount++
	} else {
		metrics.MessagesHandled++
		// 更新平均延迟（简单移动平均）
		if metrics.MessagesHandled == 1 {
			metrics.AverageLatency = latency
		} else {
			metrics.AverageLatency = (metrics.AverageLatency + latency) / 2
		}
	}
}

func (m *metricsCollector) getActorKey(actor ActorRef) string {
	// 处理nil ActorRef的情况
	if actor == nil {
		return "<nil>"
	}
	// 使用Actor引用的字符串表示作为key
	return actor.String()
}

// internalMetrics 内部监控接口，包含系统级功能（严格内部使用，不对外暴露）
type internalMetrics interface {
	// 通用消息记录方法（内部使用，区分用户和系统消息）
	RecordMessageSent(from, to ActorRef, messageType string)
	RecordMessageReceived(actor ActorRef, messageType string, latency time.Duration)

	// 系统消息记录方法（内部专用）
	recordUserMessageSent(from, to ActorRef, messageType string)
	recordUserMessageReceived(actor ActorRef, messageType string, latency time.Duration)
	recordSystemMessageSent(from, to ActorRef, messageType string)
	recordSystemMessageReceived(actor ActorRef, messageType string, latency time.Duration)
	recordMessageDeadLetter(from, to ActorRef, message Message, reason string)
	recordActorCreated(actor ActorRef, actorType string)
	recordActorTerminated(actor ActorRef, reason string)
	recordActorRestarted(actor ActorRef, reason string)

	// 控制方法（仅内部使用）
	StopRecording()
	ResumeRecording()
	IsRecording() bool

	// 获取系统指标
	getSystemSnapshot() SystemMetricsSnapshot
}

// GetCustomMetricsSnapshot 获取自定义指标的快照（用户友好的接口）
func (m *metricsCollector) GetCustomMetricsSnapshot() CustomMetricsSnapshot {
	m.customMu.RLock()
	defer m.customMu.RUnlock()

	// 创建副本以避免并发修改
	counters := make(map[string]int64)
	for k, v := range m.customCounters {
		counters[k] = v
	}

	gauges := make(map[string]float64)
	for k, v := range m.customGauges {
		gauges[k] = v
	}

	histograms := make(map[string]*HistogramData)
	for k, v := range m.customHistograms {
		copy := *v
		histograms[k] = &copy
	}

	timings := make(map[string]*TimingData)
	for k, v := range m.customTimings {
		copy := *v
		timings[k] = &copy
	}

	return CustomMetricsSnapshot{
		Timestamp:  time.Now(),
		Counters:   counters,
		Gauges:     gauges,
		Histograms: histograms,
		Timings:    timings,
	}
}
