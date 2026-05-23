package dns

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/logging"
)

type responseCache interface {
	Get(context.Context, string) *redis.StringCmd
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
}

type handler struct {
	store  *store
	logger logging.Logger
	tracer trace.Tracer
	cache  responseCache
}
