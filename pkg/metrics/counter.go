package metrics

import "sync/atomic"

// Counter 表示计数器指标，只能递增。
type Counter interface {
	// Inc 将计数器增加 1。
	Inc()

	// Add 将计数器增加指定的值。
	Add(delta uint64)
}

// atomicCounter 是线程安全的计数器实现。
type atomicCounter struct {
	value atomic.Uint64
}

func (c *atomicCounter) Inc() {
	c.value.Add(1)
}

func (c *atomicCounter) Add(delta uint64) {
	c.value.Add(delta)
}

func (c *atomicCounter) Value() uint64 {
	return c.value.Load()
}
