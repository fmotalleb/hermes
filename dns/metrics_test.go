package dns

import (
	"context"
	"net"
	"testing"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/fmotalleb/hermes/cache"
)

// testResponseWriter is a minimal dns.ResponseWriter stub.
type testResponseWriter struct{}

func (w *testResponseWriter) LocalAddr() net.Addr       { return nil }
func (w *testResponseWriter) RemoteAddr() net.Addr      { return nil }
func (w *testResponseWriter) WriteMsg(*dns.Msg) error   { return nil }
func (w *testResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (w *testResponseWriter) Close() error              { return nil }
func (w *testResponseWriter) TsigStatus() error         { return nil }
func (w *testResponseWriter) TsigTimersOnly(bool)       {}
func (w *testResponseWriter) Hijack()                   {}

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

// TestServeDNSRecordsCacheHitMetric verifies a query answered from the DNS
// response cache increments dns.cache.hits, dns.queries and dns.responses.
func TestServeDNSRecordsCacheHitMetric(t *testing.T) {
	prev := otel.GetMeterProvider()
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(prev)

	h := &handler{
		cache:      cache.NewMemoryCache(context.Background()),
		cacheTypes: []uint16{dns.TypeA},
		tracer:     otel.Tracer("test"),
	}
	// Force instrument creation against the test provider before serving.
	h.m()

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Authoritative = true
	resp.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("192.0.2.1").To4(),
		},
	}
	h.cacheResponse(context.Background(), req, resp)

	h.ServeDNS(&testResponseWriter{}, req)

	rm := &metricdata.ResourceMetrics{}
	if err := reader.Collect(context.Background(), rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	if got := counterValueByAttrs(t, rm, "dns.cache.hits", map[string]string{"qtype": "A"}); got != 1 {
		t.Errorf("expected 1 dns cache hit, got %d", got)
	}
	if got := counterValueByAttrs(t, rm, "dns.cache.misses", map[string]string{"qtype": "A"}); got != 0 {
		t.Errorf("expected 0 dns cache misses, got %d", got)
	}
	if got := counterValueByAttrs(t, rm, "dns.queries", map[string]string{"qtype": "A"}); got != 1 {
		t.Errorf("expected 1 dns query, got %d", got)
	}
	if got := counterValueByAttrs(t, rm, "dns.responses", map[string]string{"qtype": "A", "rcode": "NOERROR"}); got != 1 {
		t.Errorf("expected 1 NOERROR response, got %d", got)
	}
}
