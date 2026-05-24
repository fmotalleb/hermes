package dns

import (
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/logging"

	"github.com/fmotalleb/hermes/cache"
)

type handler struct {
	store  *store
	logger logging.Logger
	tracer trace.Tracer
	cache  cache.Cache
}
