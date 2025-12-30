package metrics

import (
	"sync"
	"unsafe"
)

// Metrics 定义了指标收集器的接口。
//
// 该接口提供了基本的指标类型：Counter（计数器）、Gauge（仪表盘）、Histogram（直方图）。
// 实现该接口可以收集 Actor 系统的运行指标，用于监控和诊断。
type Metrics interface {
	// Counter 返回指定名称的计数器指标。
	// 计数器只能递增，用于统计事件发生的次数。
	Counter(name string) Counter

	// Gauge 返回指定名称的仪表盘指标。
	// 仪表盘可以增减，用于表示当前值（如 Actor 数量）。
	Gauge(name string) Gauge

	// Histogram 返回指定名称的直方图指标。
	// 直方图用于记录值的分布（如耗时、大小等）。
	Histogram(name string) Histogram

	// Snapshot 返回当前所有指标的快照。
	// 用于查询和导出指标数据。
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot 表示指标的快照，包含所有指标的当前值。
type MetricsSnapshot struct {
	Counters   map[string]uint64            // 计数器指标
	Gauges     map[string]int64             // 仪表盘指标
	Histograms map[string]HistogramSnapshot // 直方图指标
}

// DefaultMetrics 是 Metrics 接口的默认实现。
type DefaultMetrics struct {
	counters   map[string]*atomicCounter
	gauges     map[string]*atomicGauge
	histograms map[string]*atomicHistogram
	mu         sync.RWMutex
}

// NewDefaultMetrics 创建一个新的默认指标收集器。
func NewDefaultMetrics() *DefaultMetrics {
	return &DefaultMetrics{
		counters:   make(map[string]*atomicCounter),
		gauges:     make(map[string]*atomicGauge),
		histograms: make(map[string]*atomicHistogram),
	}
}

func (m *DefaultMetrics) Counter(name string) Counter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.counters[name]; ok {
		return c
	}
	c := &atomicCounter{}
	m.counters[name] = c
	return c
}

func (m *DefaultMetrics) Gauge(name string) Gauge {
	m.mu.Lock()
	defer m.mu.Unlock()

	if g, ok := m.gauges[name]; ok {
		return g
	}
	g := &atomicGauge{}
	m.gauges[name] = g
	return g
}

func (m *DefaultMetrics) Histogram(name string) Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, ok := m.histograms[name]; ok {
		return h
	}
	h := newAtomicHistogram()
	m.histograms[name] = h
	return h
}

func (m *DefaultMetrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := MetricsSnapshot{
		Counters:   make(map[string]uint64),
		Gauges:     make(map[string]int64),
		Histograms: make(map[string]HistogramSnapshot),
	}

	for name, counter := range m.counters {
		snapshot.Counters[name] = counter.Value()
	}

	for name, gauge := range m.gauges {
		snapshot.Gauges[name] = gauge.Value()
	}

	for name, histogram := range m.histograms {
		snapshot.Histograms[name] = histogram.Snapshot()
	}

	return snapshot
}

// float64ToUint64 将 float64 转换为 uint64（用于原子操作）。
func float64ToUint64(f float64) uint64 {
	return *(*uint64)(unsafe.Pointer(&f))
}

// uint64ToFloat64 将 uint64 转换为 float64。
func uint64ToFloat64(u uint64) float64 {
	return *(*float64)(unsafe.Pointer(&u))
}
