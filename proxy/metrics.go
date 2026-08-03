package proxy

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// proxyMetrics holds the OpenTelemetry instruments used by the proxy routers.
// It is created in NewProxy so the instruments bind to the meter provider
// configured by the runtime; without a provider (e.g. in tests) every
// instrument is a no-op.
type proxyMetrics struct {
	requests metric.Int64Counter // proxy.requests (protocol, result)
}

func newProxyMetrics() *proxyMetrics {
	meter := otel.Meter("proxy")
	requests, err := meter.Int64Counter(
		"proxy.requests",
		metric.WithDescription("Proxy requests handled by protocol and result"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		requests = noop.Int64Counter{}
	}
	return &proxyMetrics{requests: requests}
}

// recordRequest counts one proxy request by protocol ("http" or "sni") and
// result ("allowed", "blocked", "malformed" or "error").
func (p *Proxy) recordRequest(ctx context.Context, protocol, result string) {
	if p.metrics == nil {
		return
	}
	p.metrics.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("protocol", protocol),
		attribute.String("result", result),
	))
}
