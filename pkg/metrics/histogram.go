package metrics

import (
	"sync"
	"sync/atomic"
)

// Histogram 表示直方图指标，用于记录值的分布。
type Histogram interface {
	// Observe 记录一个观察值。
	Observe(value float64)
}

// atomicHistogram 是线程安全的直方图实现。
type atomicHistogram struct {
	mu      sync.RWMutex
	count   atomic.Uint64
	sum     atomic.Uint64 // 使用 uint64 存储 float64 的位模式
	min     atomic.Uint64
	max     atomic.Uint64
	values  []float64 // 可选：存储所有值用于详细分析
	maxSize int       // 限制存储的值数量，避免内存泄漏
}

// HistogramSnapshot 表示直方图的快照。
type HistogramSnapshot struct {
	Count  uint64    // 观察次数
	Sum    float64   // 总和
	Min    float64   // 最小值
	Max    float64   // 最大值
	Values []float64 // 所有观察值（可选，用于详细分析）
}

func newAtomicHistogram() *atomicHistogram {
	return &atomicHistogram{
		maxSize: 1000, // 默认最多存储 1000 个值
	}
}

func (h *atomicHistogram) Observe(value float64) {
	h.count.Add(1)

	// 更新总和
	for {
		oldSum := h.sum.Load()
		newSum := float64ToUint64(uint64ToFloat64(oldSum) + value)
		if h.sum.CompareAndSwap(oldSum, newSum) {
			break
		}
	}

	// 更新最小值
	for {
		oldMin := h.min.Load()
		if oldMin == 0 || value < uint64ToFloat64(oldMin) {
			if h.min.CompareAndSwap(oldMin, float64ToUint64(value)) {
				break
			}
		} else {
			break
		}
	}

	// 更新最大值
	for {
		oldMax := h.max.Load()
		if oldMax == 0 || value > uint64ToFloat64(oldMax) {
			if h.max.CompareAndSwap(oldMax, float64ToUint64(value)) {
				break
			}
		} else {
			break
		}
	}

	// 可选：存储值（限制大小）
	h.mu.Lock()
	if len(h.values) < h.maxSize {
		h.values = append(h.values, value)
	}
	h.mu.Unlock()
}

func (h *atomicHistogram) Snapshot() HistogramSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	snapshot := HistogramSnapshot{
		Count: h.count.Load(),
		Sum:   uint64ToFloat64(h.sum.Load()),
	}

	if h.min.Load() != 0 {
		snapshot.Min = uint64ToFloat64(h.min.Load())
	}
	if h.max.Load() != 0 {
		snapshot.Max = uint64ToFloat64(h.max.Load())
	}

	// 复制值数组
	if len(h.values) > 0 {
		snapshot.Values = make([]float64, len(h.values))
		copy(snapshot.Values, h.values)
	}

	return snapshot
}
