package metrics

import (
    "sort"
    "sync"
)

type Manager interface {
    Counter(name string, tags ...Tag) Counter
    Gauge(name string, tags ...Tag) Gauge
    Histogram(name string, provider BucketProvider, tags ...Tag) Histogram
    Reset()
    GetMetrics() Snapshot
}

func NewManagerFromConfig(config *ManagerConfiguration) Manager {
    mgr := &manager{
        config:     *config,
        counters:   make(map[string]*counter),
        gauges:     make(map[string]*gauge),
        histograms: make(map[string]*histogram),
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
    counters   map[string]*counter
    gauges     map[string]*gauge
    histograms map[string]*histogram
    mu         sync.RWMutex
}

func (p *manager) GetMetrics() Snapshot {
    return newSnapshot(p)
}

func (p *manager) Counter(name string, tags ...Tag) Counter {
    key := p.makeKey(name, tags)

    p.mu.Lock()
    defer p.mu.Unlock()

    metric, exist := p.counters[key]
    if !exist {
        metric = &counter{
            name: name,
            tags: tags,
        }
        p.counters[key] = metric
    }

    return metric
}

func (p *manager) Gauge(name string, tags ...Tag) Gauge {
    key := p.makeKey(name, tags)

    p.mu.Lock()
    defer p.mu.Unlock()

    metric, exist := p.gauges[key]
    if !exist {
        metric = &gauge{
            name: name,
            tags: tags,
        }
        p.gauges[key] = metric
    }
    return metric
}

func (p *manager) Histogram(name string, provider BucketProvider, tags ...Tag) Histogram {
    key := p.makeKey(name, tags)

    p.mu.Lock()
    defer p.mu.Unlock()

    metric, exist := p.histograms[key]
    if !exist {
        metric = &histogram{
            name:    name,
            buckets: provider.Provide(),
            tags:    tags,
        }
        p.histograms[key] = metric
    }
    return metric
}

func (p *manager) Reset() {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.counters = make(map[string]*counter)
    p.gauges = make(map[string]*gauge)
    p.histograms = make(map[string]*histogram)
}

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
