package vivid

import "time"

func newRouterMetrics() *RouterMetrics {
	return &RouterMetrics{}
}

type RouterMetrics struct {
	MessageNum      uint64        // 累计消息数量
	LastBalanceTime time.Time     // 最后负载的时间
	LastCPUDuration time.Duration // 最后负载 CPU 持续时间
	PanicNum        uint64        // Panic 次数
	LastPanicTime   time.Time     // 最后 Panic 时间
}

func (m *RouterMetrics) reset() {
	m.MessageNum = 0
	m.LastBalanceTime = zeroTime
	m.LastCPUDuration = 0
	m.PanicNum = 0
	m.LastPanicTime = zeroTime
}

func (m *RouterMetrics) merge(metrics *RouterMetrics) {
	m.MessageNum += metrics.MessageNum
	m.LastBalanceTime = metrics.LastBalanceTime
	m.LastCPUDuration = metrics.LastCPUDuration
	m.PanicNum += metrics.PanicNum
	m.LastPanicTime = metrics.LastPanicTime
}
