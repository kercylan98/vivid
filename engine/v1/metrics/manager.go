package metrics

import (
	"sync"
	"time"
)

type Manager interface {
	// GetMetricCollector 获取指标采集器
	GetMetricCollector() *Collector
}

func NewManagerFromConfig(config *ManagerConfiguration) Manager {
	mgr := &manager{
		config:   *config,
		counters: make(map[string]*counterMetric),
		gauges:   make(map[string]*gaugeMetric),
		entries:  make([]metricEntry, 0),
	}
	mgr.collector = newCollector(mgr)
	return mgr
}

func NewManagerWithOptions(options ...ManagerOption) Manager {
	config := NewManagerConfiguration(options...)
	return NewManagerFromConfig(config)
}

func NewManagerWithConfigurators(configurators ...ManagerConfigurator) Manager {
	config := NewManagerConfiguration()
	for _, c := range configurators {
		c.Configure(config)
	}
	return NewManagerFromConfig(config)
}

// manager 指标管理器
type manager struct {
	config    ManagerConfiguration
	collector *Collector
	// 存储不同类型的指标
	counters map[string]*counterMetric
	gauges   map[string]*gaugeMetric
	entries  []metricEntry // 所有指标的历史记录
	mu       sync.RWMutex
}

func (p *manager) GetMetricCollector() *Collector {
	return p.collector
}

// processCounter 处理计数器指标
func (p *manager) processCounter(name string, value int64, tags ...Tag) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := p.makeKey(name, tags)

	// 更新或创建计数器
	if counter, exists := p.counters[key]; exists {
		counter.Value += value
	} else {
		p.counters[key] = &counterMetric{
			Name:  name,
			Value: value,
			Tags:  tags,
		}
	}

	// 记录历史
	p.addEntry(metricEntry{
		Type:      metricTypeCounter,
		Name:      name,
		Value:     value,
		Tags:      tags,
		Timestamp: time.Now(),
	})
}

// processGauge 处理瞬时值指标
func (p *manager) processGauge(name string, value float64, tags ...Tag) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := p.makeKey(name, tags)

	// 更新或创建瞬时值指标
	p.gauges[key] = &gaugeMetric{
		Name:  name,
		Value: value,
		Tags:  tags,
	}

	// 记录历史
	p.addEntry(metricEntry{
		Type:      metricTypeGauge,
		Name:      name,
		Value:     value,
		Tags:      tags,
		Timestamp: time.Now(),
	})
}

// processHistogram 处理直方图指标
func (p *manager) processHistogram(name string, value float64, tags ...Tag) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.addEntry(metricEntry{
		Type:      metricTypeHistogram,
		Name:      name,
		Value:     value,
		Tags:      tags,
		Timestamp: time.Now(),
	})
}

// processTimer 处理计时器指标
func (p *manager) processTimer(name string, duration time.Duration, tags ...Tag) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.addEntry(metricEntry{
		Type:      metricTypeTimer,
		Name:      name,
		Value:     duration,
		Tags:      tags,
		Timestamp: time.Now(),
	})
}

// processEvent 处理事件指标
func (p *manager) processEvent(name string, message string, tags ...Tag) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.addEntry(metricEntry{
		Type:      metricTypeEvent,
		Name:      name,
		Value:     message,
		Tags:      tags,
		Timestamp: time.Now(),
	})
}

// reset 重置所有指标
func (p *manager) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.counters = make(map[string]*counterMetric)
	p.gauges = make(map[string]*gaugeMetric)
	p.entries = p.entries[:0]
}

// addEntry 添加历史记录条目
func (p *manager) addEntry(entry metricEntry) {
	p.entries = append(p.entries, entry)

	// 如果超过最大记录数，删除最老的记录
	if len(p.entries) > p.config.MaxEntries {
		copy(p.entries, p.entries[1:])
		p.entries = p.entries[:len(p.entries)-1]
	}
}

// makeKey 生成指标的唯一键
func (p *manager) makeKey(name string, tags []Tag) string {
	if len(tags) == 0 {
		return name
	}

	key := name
	for _, tag := range tags {
		key += ":" + tag.Key + "=" + tag.Value
	}
	return key
}
