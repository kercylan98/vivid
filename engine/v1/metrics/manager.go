package metrics

import (
	"sort"
	"sync"
)

type Manager interface {
	Counter(name string, tags ...Tag) Counter
	Gauge(name string, tags ...Tag) Gauge
	Histogram(name string, provider BucketProvider, tags ...Tag) Histogram
}

func NewManagerFromConfig(config *ManagerConfiguration) Manager {
	mgr := &manager{
		config:     *config,
		counters:   make(map[string]Counter),
		gauges:     make(map[string]Gauge),
		histograms: make(map[string]Histogram),
	}
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
	config     ManagerConfiguration
	counters   map[string]Counter
	gauges     map[string]Gauge
	histograms map[string]Histogram
	mu         sync.RWMutex
}

func (p *manager) Counter(name string, tags ...Tag) Counter {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := p.makeKey(name, tags)

	metric, exist := p.counters[key]
	if !exist {
		metric = &counter{
			Name: name,
			Tags: tags,
		}
		p.counters[key] = metric
	}

	return metric
}

func (p *manager) Gauge(name string, tags ...Tag) Gauge {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := p.makeKey(name, tags)

	metric, exist := p.gauges[key]
	if !exist {
		metric = &gauge{
			Name: name,
			Tags: tags,
		}
		p.gauges[key] = metric
	}

	return metric
}

func (p *manager) Histogram(name string, provider BucketProvider, tags ...Tag) Histogram {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := p.makeKey(name, tags)

	metric, exist := p.histograms[key]
	if !exist {
		metric = &histogram{
			Name:    name,
			Buckets: provider.Provide(),
			Tags:    tags,
		}
		p.histograms[key] = metric
	}

	return metric
}

// reset 重置所有指标
func (p *manager) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.gauges = make(map[string]Gauge)
	p.counters = make(map[string]Counter)
	p.histograms = make(map[string]Histogram)
}

// makeKey 生成指标的唯一键

func (p *manager) makeKey(name string, tags []Tag) string {
	if len(tags) == 0 {
		return name
	}

	// 对标签排序确保一致性
	sortedTags := make([]Tag, len(tags))
	copy(sortedTags, tags)
	sort.Slice(sortedTags, func(i, j int) bool {
		return sortedTags[i].Key < sortedTags[j].Key
	})

	key := name
	for _, tag := range sortedTags {
		key += ":" + tag.Key + "=" + tag.Value
	}
	return key
}
