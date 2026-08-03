package proxy

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func counterValueByAttrs(t *testing.T, rm *metricdata.ResourceMetrics, name string, want map[string]string) int64 {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("expected Sum[int64] for %s, got %T", name, m.Data)
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

func TestProxyRecordRequest(t *testing.T) {
	prev := otel.GetMeterProvider()
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(prev)

	p, err := NewProxy("0.0.0.0", []string{"1080"}, nil, time.Minute, "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("new proxy: %v", err)
	}

	ctx := context.Background()
	p.recordRequest(ctx, "http", "allowed")
	p.recordRequest(ctx, "http", "blocked")
	p.recordRequest(ctx, "sni", "allowed")

	rm := &metricdata.ResourceMetrics{}
	if err := reader.Collect(ctx, rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	if got := counterValueByAttrs(t, rm, "proxy.requests", map[string]string{"protocol": "http", "result": "allowed"}); got != 1 {
		t.Errorf("expected 1 http allowed request, got %d", got)
	}
	if got := counterValueByAttrs(t, rm, "proxy.requests", map[string]string{"protocol": "http", "result": "blocked"}); got != 1 {
		t.Errorf("expected 1 http blocked request, got %d", got)
	}
	if got := counterValueByAttrs(t, rm, "proxy.requests", map[string]string{"protocol": "sni", "result": "allowed"}); got != 1 {
		t.Errorf("expected 1 sni allowed request, got %d", got)
	}
}
