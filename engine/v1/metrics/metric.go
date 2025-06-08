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
	Name  string
	Value atomic.Uint64
	Tags  []Tag
}

func (c *counter) Inc() Counter {
	c.Value.Add(1)
	return c
}

func (c *counter) Add(value uint64) Counter {
	c.Value.Add(value)
	return c
}

type gauge struct {
	Name  string
	Value float64
	Tags  []Tag
	rw    sync.RWMutex
}

func (g *gauge) Set(value float64) Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.Value = value
	return g
}

func (g *gauge) Inc() Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.Value += 1
	return g
}

func (g *gauge) Dec() Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.Value -= 1
	return g
}

func (g *gauge) Add(value float64) Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.Value += value
	return g
}

func (g *gauge) Sub(value float64) Gauge {
	g.rw.Lock()
	defer g.rw.Unlock()

	g.Value -= value
	return g
}

type histogram struct {
	Name    string
	Buckets []Bucket // 不同的区间桶
	Count   uint64   // 总观察次数
	Sum     float64  // 所有观察值的总和
	Tags    []Tag
	rw      sync.RWMutex
}

type Bucket struct {
	UpperBound float64       // 桶的上界
	Count      atomic.Uint64 // 该桶中的观察值数量
}

func (h *histogram) Observe(value float64) Histogram {
	h.rw.Lock()
	h.Count++
	h.Sum += value
	h.rw.Unlock()

	// 将值分配到合适的桶中
	for i := range h.Buckets {
		if value <= h.Buckets[i].UpperBound {
			h.Buckets[i].Count.Add(1)
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
