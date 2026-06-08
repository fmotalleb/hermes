package dns

import (
	"context"
	"log/slog"
	"net"
	"slices"

	"go.opentelemetry.io/otel/trace"

	"github.com/miekg/dns"

	"github.com/fmotalleb/hermes/cache"
	"github.com/fmotalleb/hermes/models"
)

type dnsStore interface {
	findZone(ctx context.Context, qname string) (zoneRow, error)
	loadSettings(ctx context.Context) (settingsRow, error)
	findForwardZone(ctx context.Context, id string) (forwardZoneRow, error)
	recordsForZone(ctx context.Context, zoneID string) ([]record, error)
	lookupHijacks(ctx context.Context, qtype models.DNSRecordType) ([]hijackRow, bool)
	getProxyServices(ctx context.Context) ([]net.IPAddr, error)
}

type handler struct {
	dnsStore
	logger     *slog.Logger
	tracer     trace.Tracer
	cache      cache.Cache
	cacheTypes []uint16
}

func (h *handler) shouldCache(q dns.Question) bool {
	return slices.Contains(h.cacheTypes, q.Qtype)
}
