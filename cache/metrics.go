package cache

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// metricsCache decorates a Cache with an OpenTelemetry counter so every cache
// operation — hit, miss, set, invalidation, ... — is observable in Prometheus
// and via OTLP metric push, no matter which backend is in use. It is applied
// at construction time in NewMemoryCache and NewRedisCache; without a meter
// provider configured (e.g. in tests) the instrument is a no-op.
type metricsCache struct {
	inner      Cache
	operations metric.Int64Counter
}

// withMetrics wraps inner so cache operations are counted, following the
// OpenTelemetry cache semantic conventions (operation and result attributes).
func withMetrics(inner Cache) Cache {
	meter := otel.Meter("cache")
	operations, err := meter.Int64Counter(
		"cache.operations",
		metric.WithDescription("Number of cache operations by operation and result"),
		metric.WithUnit("{operation}"),
	)
	if err != nil {
		operations = noop.Int64Counter{}
	}
	return &metricsCache{inner: inner, operations: operations}
}

func (c *metricsCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	data, err := c.inner.GetBytes(ctx, key)
	result := "hit"
	switch {
	case err == nil:
	case isCacheMiss(err):
		result = "miss"
	default:
		result = "error"
	}
	c.operations.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache.operation", "get"),
		attribute.String("cache.operation.result", result),
	))
	return data, err
}

func (c *metricsCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	err := c.inner.Set(ctx, key, value, ttl)
	c.record(ctx, "set", err)
	return err
}

func (c *metricsCache) Clear(ctx context.Context) error {
	err := c.inner.Clear(ctx)
	c.record(ctx, "clear", err)
	return err
}

// Delete implements [Cache].
func (c *metricsCache) Delete(ctx context.Context, key string) error {
	err := c.inner.Delete(ctx, key)
	c.record(ctx, "del", err)
	return err
}

// DeleteSelector implements [Cache].
func (c *metricsCache) DeleteSelector(ctx context.Context, selector func(string) bool) error {
	err := c.inner.DeleteSelector(ctx, selector)
	c.record(ctx, "del_selector", err)
	return err
}

// DeletePattern implements [Cache].
func (c *metricsCache) DeletePattern(ctx context.Context, pattern string) error {
	err := c.inner.DeletePattern(ctx, pattern)
	c.record(ctx, "del_pattern", err)
	return err
}

func (c *metricsCache) record(ctx context.Context, operation string, err error) {
	result := "value"
	if err != nil {
		result = "error"
	}
	c.operations.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache.operation", operation),
		attribute.String("cache.operation.result", result),
	))
}
