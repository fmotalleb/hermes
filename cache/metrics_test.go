package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// counterValueByAttrs sums cache.operations data points whose attributes
// contain every key/value in want.
func counterValueByAttrs(t *testing.T, rm *metricdata.ResourceMetrics, want map[string]string) int64 {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "cache.operations" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("expected Sum[int64] for %s, got %T", m.Name, m.Data)
			}
			for _, dp := range sum.DataPoints {
				if attrsContain(dp.Attributes.ToSlice(), want) {
					return dp.Value
				}
			}
		}
	}
	return 0
}

func attrsContain(attrs []attribute.KeyValue, want map[string]string) bool {
	for k, v := range want {
		found := false
		for _, a := range attrs {
			if string(a.Key) == k && a.Value.AsString() == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestMetricsCacheCountsOperations(t *testing.T) {
	prev := otel.GetMeterProvider()
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(prev)

	ctx := context.Background()
	c := NewMemoryCache(ctx)

	// miss
	if _, err := c.GetBytes(ctx, "missing"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected miss, got %v", err)
	}
	// set then hit
	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := c.GetBytes(ctx, "k"); err != nil {
		t.Fatalf("get: %v", err)
	}
	// failing set (unsupported type)
	if err := c.Set(ctx, "k2", "not-bytes", time.Minute); !errors.Is(err, ErrStoreUnsupportedType) {
		t.Fatalf("expected unsupported type error, got %v", err)
	}
	// clear
	if err := c.Clear(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}

	rm := &metricdata.ResourceMetrics{}
	if err := reader.Collect(ctx, rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	get := func(result string) int64 {
		return counterValueByAttrs(t, rm, map[string]string{
			"cache.operation":        "get",
			"cache.operation.result": result,
		})
	}
	if got := get("hit"); got != 1 {
		t.Errorf("expected 1 get hit, got %d", got)
	}
	if got := get("miss"); got != 1 {
		t.Errorf("expected 1 get miss, got %d", got)
	}
	if got := counterValueByAttrs(t, rm, map[string]string{
		"cache.operation":        "set",
		"cache.operation.result": "value",
	}); got != 1 {
		t.Errorf("expected 1 successful set, got %d", got)
	}
	if got := counterValueByAttrs(t, rm, map[string]string{
		"cache.operation":        "set",
		"cache.operation.result": "error",
	}); got != 1 {
		t.Errorf("expected 1 failed set, got %d", got)
	}
	if got := counterValueByAttrs(t, rm, map[string]string{
		"cache.operation":        "clear",
		"cache.operation.result": "value",
	}); got != 1 {
		t.Errorf("expected 1 clear, got %d", got)
	}
}
