package metrics

import (
	"math"
	"sync"
	"sync/atomic"
)

type Counter interface {
	Inc() Counter
	Add(value uint64) Counter
}

type Gauge interface {
	Set(value float64) Gauge
	Inc() Gauge
	Dec() Gauge
	Add(value float64) Gauge
	Sub(value float64) Gauge
}

type Histogram interface {
	Observe(value float64) Histogram
}

type BucketProvider interface {
	Provide() []Bucket
}

type HistogramBucketProviderFN func() []Bucket

func (f HistogramBucketProviderFN) Provide() []Bucket {
	return f()
}

type counter struct {
	name  string
	value atomic.Uint64
	tags  []Tag
}

func (c *counter) snapshot() CounterSnapshot {
	snapshot := CounterSnapshot{
		Name:  c.name,
		Value: c.value.Load(),
		Tags:  c.tags,
	}

	return snapshot
}

type CounterSnapshot struct {
	Name  string `json:"name"`
	Value uint64 `json:"value"`
	Tags  []Tag  `json:"tags"`
}

func (c *counter) Inc() Counter {
	c.value.Add(1)
	return c
}

func (c *counter) Add(value uint64) Counter {
	c.value.Add(value)
	return c
}

type gauge struct {
	name  string
	value float64
	tags  []Tag
	rw    sync.RWMutex
}

func (g *gauge) snapshot() GaugeSnapshot {
	g.rw.RLock()
	defer g.rw.RUnlock()

	snapshot := GaugeSnapshot{
		Name:  g.name,
		Value: g.value,
		Tags:  g.tags,
	}

	return snapshot
}

type GaugeSnapshot struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Tags  []Tag   `json:"tags"`
}

func (g *gauge) Set(value float64) Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.value = value
	return g
}

func (g *gauge) Inc() Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.value += 1
	return g
}

func (g *gauge) Dec() Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.value -= 1
	return g
}

func (g *gauge) Add(value float64) Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.value += value
	return g
}

func (g *gauge) Sub(value float64) Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.value -= value
	return g
}

type histogram struct {
	name    string
	buckets []Bucket // 不同的区间桶
	count   uint64   // 总观察次数
	sum     float64  // 所有观察值的总和
	tags    []Tag
	rw      sync.RWMutex
}

func (h *histogram) snapshot() HistogramSnapshot {
	h.rw.RLock()
	defer h.rw.RUnlock()

	snapshot := HistogramSnapshot{
		Name:    h.name,
		Count:   h.count,
		Sum:     h.sum,
		Buckets: make([]BucketSnapshot, len(h.buckets)),
		Tags:    h.tags,
	}
	for i := range h.buckets {
		snapshot.Buckets[i] = BucketSnapshot{
			UpperBound: h.buckets[i].UpperBound,
			Count:      h.buckets[i].count.Load(),
		}
	}
	return snapshot
}

type Bucket struct {
	UpperBound float64       // 桶的上界
	count      atomic.Uint64 // 该桶中的观察值数量
}

type HistogramSnapshot struct {
	Name    string           `json:"name"`
	Count   uint64           `json:"count"`
	Sum     float64          `json:"sum"`
	Buckets []BucketSnapshot `json:"buckets"`
	Tags    []Tag            `json:"tags"`
}

type BucketSnapshot struct {
	UpperBound float64 `json:"upper_bound"`
	Count      uint64  `json:"count"`
}

func (h *histogram) Observe(value float64) Histogram {
	h.rw.Lock()
	h.count++
	h.sum += value
	h.rw.Unlock()

	// 将值分配到合适的桶中
	for i := range h.buckets {
		if value <= h.buckets[i].UpperBound {
			h.buckets[i].count.Add(1)
		}
	}
	return h
}

// ExponentialBuckets 生成指数增长的桶边界
func ExponentialBuckets(start, factor float64, count int) []Bucket {
	buckets := make([]Bucket, count+1)
	for i := 0; i < count; i++ {
		buckets[i] = Bucket{UpperBound: start * math.Pow(factor, float64(i))}
	}
	buckets[count] = Bucket{UpperBound: math.Inf(1)}
	return buckets
}

// LinearBuckets 生成线性分布的桶边界
func LinearBuckets(start, width float64, count int) []Bucket {
	buckets := make([]Bucket, count+1)
	for i := 0; i < count; i++ {
		buckets[i] = Bucket{UpperBound: start + float64(i)*width}
	}
	buckets[count] = Bucket{UpperBound: math.Inf(1)}
	return buckets
}
