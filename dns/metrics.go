package dns

import (
	"context"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// dnsMetrics holds the OpenTelemetry instruments used by the DNS server. It is
// created lazily (see handler.m) so the instruments bind to the meter provider
// configured by the runtime; without a provider (e.g. in tests) every
// instrument is a no-op and recording is free.
type dnsMetrics struct {
	queries         metric.Int64Counter // dns.queries (qtype)
	cacheHits       metric.Int64Counter // dns.cache.hits (qtype)
	cacheMisses     metric.Int64Counter // dns.cache.misses (qtype)
	responses       metric.Int64Counter // dns.responses (qtype, rcode)
	lookupDuration  metric.Float64Histogram
	forwardAttempts metric.Int64Counter // dns.forward.attempts (protocol, result)
	hijacks         metric.Int64Counter // dns.hijacks (policy)
	errors          metric.Int64Counter // dns.errors (qtype)
}

// newDNSMetrics creates the DNS instruments from the global meter provider.
func newDNSMetrics() *dnsMetrics {
	meter := otel.Meter("dns")
	return &dnsMetrics{
		queries:         mustCounter(meter, "dns.queries", "DNS queries received", "{query}"),
		cacheHits:       mustCounter(meter, "dns.cache.hits", "DNS response cache hits", "{hit}"),
		cacheMisses:     mustCounter(meter, "dns.cache.misses", "DNS response cache misses", "{miss}"),
		responses:       mustCounter(meter, "dns.responses", "DNS responses sent", "{response}"),
		lookupDuration:  mustHistogram(meter, "dns.lookup.duration", "Time taken to perform a DNS lookup", "s"),
		forwardAttempts: mustCounter(meter, "dns.forward.attempts", "Forward attempts by protocol and result", "{attempt}"),
		hijacks:         mustCounter(meter, "dns.hijacks", "Queries answered by a hijack rule", "{hijack}"),
		errors:          mustCounter(meter, "dns.errors", "DNS lookups that failed", "{error}"),
	}
}

func recordResponse(m *dnsMetrics, ctx context.Context, qtype string, rcode int) {
	m.responses.Add(ctx, 1, metric.WithAttributes(
		attribute.String("qtype", qtype),
		attribute.String("rcode", dns.RcodeToString[rcode]),
	))
}

func recordForwardAttempt(m *dnsMetrics, ctx context.Context, protocol, result string) {
	m.forwardAttempts.Add(ctx, 1, metric.WithAttributes(
		attribute.String("protocol", protocol),
		attribute.String("result", result),
	))
}

// mustCounter creates an Int64Counter, falling back to a no-op instrument when
// the name or options are invalid so the server keeps running.
func mustCounter(m metric.Meter, name, desc, unit string) metric.Int64Counter {
	c, err := m.Int64Counter(name, metric.WithDescription(desc), metric.WithUnit(unit))
	if err != nil {
		return noop.Int64Counter{}
	}
	return c
}

// mustHistogram creates a Float64Histogram, falling back to a no-op instrument
// when the name or options are invalid so the server keeps running.
func mustHistogram(m metric.Meter, name, desc, unit string) metric.Float64Histogram {
	hist, err := m.Float64Histogram(name, metric.WithDescription(desc), metric.WithUnit(unit))
	if err != nil {
		return noop.Float64Histogram{}
	}
	return hist
}
