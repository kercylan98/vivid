package metrics

import "time"

const (
    metricTypeHistogram = "histogram"
    metricTypeCounter   = "counter"
    metricTypeTimer     = "timer"
    metricTypeEvent     = "event"
    metricTypeGauge     = "gauge"
)

type metricType = string

// metricEntry 指标条目
type metricEntry struct {
    Type      metricType `json:"type"`
    Name      string     `json:"name"`
    Value     any        `json:"value"`
    Tags      []Tag      `json:"tags"`
    Timestamp time.Time  `json:"timestamp"`
}

// counterMetric 计数器指标
type counterMetric struct {
    Name  string
    Value int64
    Tags  []Tag
}

// gaugeMetric 瞬时值指标
type gaugeMetric struct {
    Name  string
    Value float64
    Tags  []Tag
}
