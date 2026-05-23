package dns

import (
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/logging"
)

type handler struct {
	store  *store
	logger logging.Logger
	ctx    *gofr.Context
	tracer trace.Tracer
}
