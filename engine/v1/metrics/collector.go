package metrics

import (
	"time"
)

// Collector 指标采集器
type Collector struct {
	manager *manager
}

// newCollector 创建新的指标采集器
func newCollector(manager *manager) *Collector {
	return &Collector{
		manager: manager,
	}
}

// Counter 记录计数器指标
func (c *Collector) Counter(name string, value int64, tags ...Tag) {
	c.manager.processCounter(name, value, tags...)
}

// Gauge 记录瞬时值指标
func (c *Collector) Gauge(name string, value float64, tags ...Tag) {
	c.manager.processGauge(name, value, tags...)
}

// Histogram 记录直方图指标
func (c *Collector) Histogram(name string, value float64, tags ...Tag) {
	c.manager.processHistogram(name, value, tags...)
}

// Timer 记录计时器指标
func (c *Collector) Timer(name string, duration time.Duration, tags ...Tag) {
	c.manager.processTimer(name, duration, tags...)
}

// Event 记录事件指标
func (c *Collector) Event(name string, message string, tags ...Tag) {
	c.manager.processEvent(name, message, tags...)
}
