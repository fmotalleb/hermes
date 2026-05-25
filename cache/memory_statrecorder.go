package cache

import (
	"context"
	"time"

	"github.com/maypok86/otter/v2/stats"
	"gofr.dev/pkg/gofr/metrics"
)

const (
	metricHits = "memory_cache_hits_total"

	metricMisses = "memory_cache_misses_total"

	metricEvictedWeight = "memory_cache_evicted_weight_total"

	metricLoadSuccess = "memory_cache_load_success_total"

	metricLoadFailure = "memory_cache_load_failure_total"

	metricLoadDuration = "memory_cache_load_duration_seconds"
)

type memCacheMetrics struct {
	m   metrics.Manager
	ctx context.Context
}

func newMemoryCacheRecorder(ctx context.Context, m metrics.Manager) stats.Recorder {
	r := &memCacheMetrics{
		m:   m,
		ctx: ctx,
	}

	r.register()

	return r
}

func (m *memCacheMetrics) register() {
	m.m.NewUpDownCounter(
		metricHits,
		"Total number of cache hits",
	)

	m.m.NewUpDownCounter(
		metricMisses,
		"Total number of cache misses",
	)

	m.m.NewUpDownCounter(
		metricEvictedWeight,
		"Total evicted cache weight",
	)

	m.m.NewCounter(
		metricLoadSuccess,
		"Total successful cache loads",
	)

	m.m.NewCounter(
		metricLoadFailure,
		"Total failed cache loads",
	)

	m.m.NewHistogram(
		metricLoadDuration,
		"Cache load duration in seconds",
		0.0001,
		0.0005,
		0.001,
		0.005,
		0.01,
		0.05,
		0.1,
		0.5,
		1,
		5,
	)
}

func (m *memCacheMetrics) RecordEviction(weight uint32) {
	if weight == 0 {
		return
	}

	m.m.DeltaUpDownCounter(
		m.ctx,
		metricEvictedWeight,
		float64(weight),
	)
}

func (m *memCacheMetrics) RecordHits(count int) {
	if count <= 0 {
		return
	}

	m.m.DeltaUpDownCounter(
		m.ctx,
		metricHits,
		float64(count),
	)
}

func (m *memCacheMetrics) RecordMisses(count int) {
	if count <= 0 {
		return
	}

	m.m.DeltaUpDownCounter(
		m.ctx,
		metricMisses,
		float64(count),
	)
}

func (m *memCacheMetrics) RecordLoadSuccess(loadTime time.Duration) {
	m.m.IncrementCounter(
		m.ctx,
		metricLoadSuccess,
	)

	m.m.RecordHistogram(
		m.ctx,
		metricLoadDuration,
		loadTime.Seconds(),
		"result",
		"success",
	)
}

func (m *memCacheMetrics) RecordLoadFailure(loadTime time.Duration) {
	m.m.IncrementCounter(
		m.ctx,
		metricLoadFailure,
	)

	m.m.RecordHistogram(
		m.ctx,
		metricLoadDuration,
		loadTime.Seconds(),
		"result",
		"failure",
	)
}
