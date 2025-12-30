package metrics

import "sync/atomic"

// Gauge 表示仪表盘指标，可以增减。
type Gauge interface {
	// Set 设置仪表盘的当前值。
	Set(value int64)

	// Inc 将仪表盘增加 1。
	Inc()

	// Dec 将仪表盘减少 1。
	Dec()

	// Add 将仪表盘增加指定的值。
	Add(delta int64)

	// Sub 将仪表盘减少指定的值。
	Sub(delta int64)
}

// atomicGauge 是线程安全的仪表盘实现。
type atomicGauge struct {
	value atomic.Int64
}

func (g *atomicGauge) Set(value int64) {
	g.value.Store(value)
}

func (g *atomicGauge) Inc() {
	g.value.Add(1)
}

func (g *atomicGauge) Dec() {
	g.value.Add(-1)
}

func (g *atomicGauge) Add(delta int64) {
	g.value.Add(delta)
}

func (g *atomicGauge) Sub(delta int64) {
	g.value.Add(-delta)
}

func (g *atomicGauge) Value() int64 {
	return g.value.Load()
}
