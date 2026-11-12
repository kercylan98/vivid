package actor

import "github.com/kercylan98/vivid/pkg/metrics"

var (
	_ metrics.Metrics   = (*discordMetrics)(nil)
	_ metrics.Counter   = (*discordMetricsCounter)(nil)
	_ metrics.Gauge     = (*discordMetricsGauge)(nil)
	_ metrics.Histogram = (*discordMetricsHistogram)(nil)

	_discordMetrics          = new(discordMetrics)
	_discordMetricsCounter   = new(discordMetricsCounter)
	_discordMetricsGauge     = new(discordMetricsGauge)
	_discordMetricsHistogram = new(discordMetricsHistogram)
)

func getDiscordMetrics() *discordMetrics {
	return _discordMetrics
}

type discordMetrics struct{}

// Counter implements [metrics.Metrics].
func (d *discordMetrics) Counter(name string) metrics.Counter {
	return _discordMetricsCounter
}

// Gauge implements [metrics.Metrics].
func (d *discordMetrics) Gauge(name string) metrics.Gauge {
	return _discordMetricsGauge
}

// Histogram implements [metrics.Metrics].
func (d *discordMetrics) Histogram(name string) metrics.Histogram {
	return _discordMetricsHistogram
}

// Snapshot implements [metrics.Metrics].
func (d *discordMetrics) Snapshot() (snapshot metrics.MetricsSnapshot) {
	return
}

type discordMetricsCounter struct{}

// Add implements [metrics.Counter].
func (d *discordMetricsCounter) Add(delta uint64) {}

// Inc implements [metrics.Counter].
func (d *discordMetricsCounter) Inc() {}

type discordMetricsGauge struct{}

// Add implements [metrics.Gauge].
func (d *discordMetricsGauge) Add(delta int64) {}

// Dec implements [metrics.Gauge].
func (d *discordMetricsGauge) Dec() {}

// Inc implements [metrics.Gauge].
func (d *discordMetricsGauge) Inc() {}

// Set implements [metrics.Gauge].
func (d *discordMetricsGauge) Set(value int64) {}

// Sub implements [metrics.Gauge].
func (d *discordMetricsGauge) Sub(delta int64) {}

type discordMetricsHistogram struct{}

// Observe implements [metrics.Histogram].
func (d *discordMetricsHistogram) Observe(value float64) {}
