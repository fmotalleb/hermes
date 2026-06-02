package dns

import (
	"log/slog"
	"slices"

	"go.opentelemetry.io/otel/trace"

	"github.com/miekg/dns"

	"github.com/fmotalleb/hermes/cache"
)

type handler struct {
	store      *store
	logger     *slog.Logger
	tracer     trace.Tracer
	cache      cache.Cache
	cacheTypes []uint16
}

func (h *handler) shouldCache(q dns.Question) bool {
	return slices.Contains(h.cacheTypes, q.Qtype)
}
