package metrics

import (
	"math"
)

type Counter interface {
	Inc()
	Add(value uint64)
}

type Gauge interface {
	Set(value float64)
	Inc()
	Dec()
	Add(value float64)
	Sub(value float64)
}

type Histogram interface {
	Observe(value float64)
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
	Value uint64
	Tags  []Tag
}

func (c *counter) Inc() {
	c.Value++
}

func (c *counter) Add(value uint64) {
	c.Value += value
}

type gauge struct {
	Name  string
	Value float64
	Tags  []Tag
}

func (g *gauge) Set(value float64) {
	g.Value = value
}

func (g *gauge) Inc() {
	g.Value += 1
}

func (g *gauge) Dec() {
	g.Value -= 1
}

func (g *gauge) Add(value float64) {
	g.Value += value
}

func (g *gauge) Sub(value float64) {
	g.Value -= value
}

type histogram struct {
	Name    string
	Buckets []Bucket // 不同的区间桶
	Count   uint64   // 总观察次数
	Sum     float64  // 所有观察值的总和
	Tags    []Tag
}

type Bucket struct {
	UpperBound float64 // 桶的上界
	Count      uint64  // 该桶中的观察值数量
}

func (h *histogram) Observe(value float64) {
	h.Count++
	h.Sum += value

	// 将值分配到合适的桶中
	for i := range h.Buckets {
		if value <= h.Buckets[i].UpperBound {
			h.Buckets[i].Count++
		}
	}
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
