package metrics

import (
	"time"
)

func newSnapshot(manager *manager) Snapshot {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	now := time.Now()
	snapshot := Snapshot{
		Timestamp:     now,
		TimestampUnix: now.Unix(),
		Counters:      make(map[string]CounterSnapshot),
		Gauges:        make(map[string]GaugeSnapshot),
		Histograms:    make(map[string]HistogramSnapshot),
	}

	for _, counter := range manager.counters {
		snapshot.Counters[counter.name] = counter.snapshot()
	}

	for _, gauge := range manager.gauges {
		snapshot.Gauges[gauge.name] = gauge.snapshot()
	}

	for _, histogram := range manager.histograms {
		snapshot.Histograms[histogram.name] = histogram.snapshot()
	}
	return snapshot
}

type Snapshot struct {
	Timestamp     time.Time `json:"timestamp"`
	TimestampUnix int64     `json:"timestamp_unix"`

	Counters   map[string]CounterSnapshot   `json:"counters"`
	Gauges     map[string]GaugeSnapshot     `json:"gauges"`
	Histograms map[string]HistogramSnapshot `json:"histograms"`
}
