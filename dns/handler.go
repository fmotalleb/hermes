package dns

import (
	"context"
	"net"
	"slices"
	"sync"

	"github.com/miekg/dns"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/models"
)

type dnsStore interface {
	findZone(ctx context.Context, qname string) (zoneRow, error)
	loadSettings(ctx context.Context) (settingsRow, error)
	findForwardZone(ctx context.Context, id string) (forwardZoneRow, error)
	recordsForZone(ctx context.Context, zoneID string) ([]record, error)
	lookupHijacks(ctx context.Context, qtype models.DNSRecordType) ([]hijackRow, bool)
	getProxyServices(ctx context.Context) ([]net.IP, error)
}

type handler struct {
	dnsStore
	logger     *zap.Logger
	tracer     trace.Tracer
	cache      cache.Cache
	cacheTypes []uint16

	metricsOnce sync.Once
	metrics     *dnsMetrics
}

// m returns the handler's metric instruments, creating them on first use so
// they bind to the meter provider configured by the runtime. When no provider
// is configured (e.g. in tests) the instruments are no-ops.
func (h *handler) m() *dnsMetrics {
	h.metricsOnce.Do(func() {
		h.metrics = newDNSMetrics()
	})
	return h.metrics
}

// log returns the handler logger, or a nop logger when none was configured
// (e.g. in tests that construct a bare handler).
func (h *handler) log() *zap.Logger {
	if h.logger == nil {
		return zap.NewNop()
	}
	return h.logger
}

func (h *handler) shouldCache(q dns.Question) bool {
	return slices.Contains(h.cacheTypes, q.Qtype)
}
