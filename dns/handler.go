package dns

import (
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/fmotalleb/hermes/cache"
)

type handler struct {
	store  *store
	logger *slog.Logger
	tracer trace.Tracer
	cache  cache.Cache
}
