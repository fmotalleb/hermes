package otellog

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fmotalleb/go-tools/log"
)

// Config holds the knobs for the OTLP log pipeline. Zero values fall back to
// the OpenTelemetry SDK defaults so callers only need to set what they care
// about.
type Config struct {
	// URL is the LOG_URL value: a bare host:port endpoint or a full
	// http(s):// URL. An empty URL disables the integration.
	URL     string
	Headers map[string]string

	// Batch processor tuning (sdklog.BatchProcessor).
	QueueSize        int           // max queued records (default 2048)
	ExportInterval   time.Duration // how often a batch is exported (default 1s)
	ExportTimeout    time.Duration // timeout for each export (default 30s)
	MaxBatchSize     int           // max records per exported batch (default 512)
	ExportBufferSize int           // per-export copy buffer (default 1)

	// Exporter tuning (otlploghttp).
	ExporterTimeout time.Duration // per-request timeout (default 10s)
	MaxRequestSize  int           // max OTLP request body size (default 64 MiB)
	Compression     string        // "gzip" enables gzip compression
}

// Integrate bridges the zap logger attached to ctx (see [log.WithLogger]) to an
// OTLP log collector so every record emitted through log.FromContext is
// exported as well. cfg.URL empty disables the integration and returns ctx
// untouched with a nil provider.
//
// The returned context carries a logger that tees every record to the base
// logger and to the collector. The caller must shut the returned provider down
// when the application exits so pending logs are flushed.
func Integrate(ctx context.Context, cfg Config) (context.Context, *sdklog.LoggerProvider, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return ctx, nil, nil
	}

	otlpExporter, err := otlploghttp.New(ctx, exporterOptions(cfg)...)
	if err != nil {
		return ctx, nil, fmt.Errorf("create otlp log exporter: %w", err)
	}
	// Surface export failures (unreachable collector, rejected requests, ...)
	// on the console instead of dropping them silently in the batch processor.
	var exporter sdklog.Exporter = &loggingExporter{Exporter: otlpExporter, logger: log.Of(ctx).Named("otellog")}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(exporter, batchProcessorOptions(cfg)...),
		),
		sdklog.WithResource(resource.DefaultWithContext(ctx)),
	)

	core := otelzap.NewCore(
		"github.com/fmotalleb/hermes",
		otelzap.WithLoggerProvider(provider),
	)
	logger := log.Of(ctx).WithOptions(zap.WrapCore(func(existing zapcore.Core) zapcore.Core {
		return zapcore.NewTee(existing, core)
	}))

	return log.WithLogger(ctx, logger), provider, nil
}

// exporterOptions builds the otlploghttp options from the configured URL,
// headers, and tuning values. A value with an explicit http(s) scheme is passed
// through as a full URL; anything else is treated as a host:port endpoint.
func exporterOptions(cfg Config) []otlploghttp.Option {
	url := logExportURL(cfg.URL)
	opts := []otlploghttp.Option{}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		opts = append(opts, otlploghttp.WithEndpointURL(url))
	} else {
		opts = append(opts, otlploghttp.WithEndpoint(url))
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlploghttp.WithHeaders(cfg.Headers))
	}
	if cfg.ExporterTimeout > 0 {
		opts = append(opts, otlploghttp.WithTimeout(cfg.ExporterTimeout))
	}
	if cfg.MaxRequestSize > 0 {
		opts = append(opts, otlploghttp.WithMaxRequestSize(cfg.MaxRequestSize))
	}
	if strings.EqualFold(cfg.Compression, "gzip") {
		opts = append(opts, otlploghttp.WithCompression(otlploghttp.GzipCompression))
	}
	return opts
}

// loggingExporter wraps an sdklog.Exporter and reports export failures through
// the given logger. Without it, a misconfigured or unreachable collector fails
// silently inside the batch processor and the logs simply vanish.
//
// The OTel SDK has no error-handler hook for log export in this version, so the
// wrap is the way to surface failures on the console.
type loggingExporter struct {
	sdklog.Exporter
	logger *zap.Logger
}

func (l *loggingExporter) Export(ctx context.Context, records []sdklog.Record) error {
	if err := l.Exporter.Export(ctx, records); err != nil {
		l.logger.Warn("failed to export otel logs", zap.Int("records", len(records)), zap.Error(err))
		return err
	}
	return nil
}

// logExportURL normalizes the configured collector URL into the OTLP log
// endpoint: the default /v1/logs path is appended only when the URL does not
// already carry an explicit path (mirroring the trace exporter's path
// handling).
func logExportURL(raw string) string {
	raw = strings.TrimRight(raw, "/")
	if !strings.Contains(raw, "://") {
		return raw + "/v1/logs"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw + "/v1/logs"
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/v1/logs"
	}
	return u.String()
}

// batchProcessorOptions maps the configured tuning values to
// sdklog.BatchProcessor options, skipping any zero values so the SDK defaults
// apply.
func batchProcessorOptions(cfg Config) []sdklog.BatchProcessorOption {
	opts := []sdklog.BatchProcessorOption{}
	if cfg.QueueSize > 0 {
		opts = append(opts, sdklog.WithMaxQueueSize(cfg.QueueSize))
	}
	if cfg.ExportInterval > 0 {
		opts = append(opts, sdklog.WithExportInterval(cfg.ExportInterval))
	}
	if cfg.ExportTimeout > 0 {
		opts = append(opts, sdklog.WithExportTimeout(cfg.ExportTimeout))
	}
	if cfg.MaxBatchSize > 0 {
		opts = append(opts, sdklog.WithExportMaxBatchSize(cfg.MaxBatchSize))
	}
	if cfg.ExportBufferSize > 0 {
		opts = append(opts, sdklog.WithExportBufferSize(cfg.ExportBufferSize))
	}
	return opts
}
