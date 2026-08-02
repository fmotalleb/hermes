package otellog

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fmotalleb/go-tools/log"
)

// Integrate bridges the zap logger attached to ctx (see [log.WithLogger]) to an
// OTLP log collector. url is the LOG_URL value and headers the parsed
// LOG_HEADERS map. An empty url disables the integration and returns ctx
// untouched with a nil provider.
//
// The returned context carries a logger that tees every record to the base
// logger and to the collector. The returned provider must be shut down when the
// application exits so pending logs are flushed.
func Integrate(ctx context.Context, url string, headers map[string]string) (context.Context, error) {
	if strings.TrimSpace(url) == "" {
		return ctx, nil
	}

	exporter, err := otlploghttp.New(ctx, otlplogOptions(url+"/v1/logs", headers)...)
	if err != nil {
		return ctx, fmt.Errorf("create otlp log exporter: %w", err)
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(exporter),
		),
		sdklog.WithResource(resource.Default()),
	)

	core := otelzap.NewCore(
		"github.com/fmotalleb/hermes",
		otelzap.WithLoggerProvider(provider),
	)
	logger := log.Of(ctx).WithOptions(zap.WrapCore(func(existing zapcore.Core) zapcore.Core {
		return zapcore.NewTee(existing, core)
	}))

	return log.WithLogger(ctx, logger), nil
}

// otlplogOptions builds the otlploghttp options from the collector URL and the
// configured headers. A value with an explicit http(s) scheme is passed through
// as a full URL; anything else is treated as a host:port endpoint.
func otlplogOptions(url string, headers map[string]string) []otlploghttp.Option {
	opts := []otlploghttp.Option{}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		opts = append(opts, otlploghttp.WithEndpointURL(url))
	} else {
		opts = append(opts, otlploghttp.WithEndpoint(url))
	}
	if len(headers) > 0 {
		opts = append(opts, otlploghttp.WithHeaders(headers))
	}
	return opts
}
