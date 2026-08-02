package otellog

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fmotalleb/go-tools/log"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLogExportURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"localhost:4318", "localhost:4318/v1/logs"},
		{"localhost:4318/", "localhost:4318/v1/logs"},
		{"http://collector:4318", "http://collector:4318/v1/logs"},
		{"http://collector:4318/", "http://collector:4318/v1/logs"},
		{"http://collector:4318/v1/logs", "http://collector:4318/v1/logs"},
		{"https://collector.example.com:4318/v1/logs", "https://collector.example.com:4318/v1/logs"},
		{"grpc://collector:4317", "grpc://collector:4317/v1/logs"},
	}
	for _, tc := range cases {
		if got := logExportURL(tc.raw); got != tc.want {
			t.Errorf("logExportURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestBatchProcessorOptions(t *testing.T) {
	// Zero config yields no options so the SDK defaults apply.
	if got := batchProcessorOptions(Config{}); len(got) != 0 {
		t.Fatalf("expected no options for zero config, got %d", len(got))
	}

	cfg := Config{
		QueueSize:        100,
		ExportInterval:   2 * time.Second,
		ExportTimeout:    5 * time.Second,
		MaxBatchSize:     10,
		ExportBufferSize: 3,
	}
	if got := batchProcessorOptions(cfg); len(got) != 5 {
		t.Fatalf("expected 5 options, got %d", len(got))
	}

	partial := Config{QueueSize: 50}
	if got := batchProcessorOptions(partial); len(got) != 1 {
		t.Fatalf("expected 1 option, got %d", len(got))
	}
}

// TestIntegrateExportsToCollector is an end-to-end check of the whole log
// pipeline: a record emitted through the context logger must reach the OTLP
// collector as an export request on /v1/logs.
func TestIntegrateExportsToCollector(t *testing.T) {
	var mu sync.Mutex
	var posts []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		posts = append(posts, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	base := log.WithLogger(context.Background(), zap.NewNop())
	ctx, provider, err := Integrate(base, Config{
		URL:            ts.URL,
		ExportInterval: 50 * time.Millisecond,
		ExportTimeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("integrate: %v", err)
	}
	defer func() { _ = provider.Shutdown(context.Background()) }()

	log.Of(ctx).Info("hello otel", zap.String("key", "value"))

	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		n := len(posts)
		mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("collector received no export request within timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(posts[0], "/v1/logs") {
		t.Fatalf("expected export to /v1/logs, got path %q", posts[0])
	}
}

// failExporter always fails, to exercise the logging wrapper.
type failExporter struct{}

func (failExporter) Export(context.Context, []sdklog.Record) error { return errors.New("boom") }
func (failExporter) Shutdown(context.Context) error                { return nil }
func (failExporter) ForceFlush(context.Context) error              { return nil }

// TestLoggingExporterSurfacesFailures verifies export failures are reported
// through the logger instead of vanishing silently.
func TestLoggingExporterSurfacesFailures(t *testing.T) {
	var buf bytes.Buffer
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&buf),
		zapcore.WarnLevel,
	))

	l := &loggingExporter{Exporter: failExporter{}, logger: logger}
	if err := l.Export(context.Background(), nil); err == nil {
		t.Fatal("expected export error")
	}
	if !strings.Contains(buf.String(), "failed to export otel logs") {
		t.Fatalf("expected failure warning in output, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Fatalf("expected underlying error in output, got %q", buf.String())
	}
}

// captureExporter records every exported log record for inspection.
type captureExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *captureExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	e.records = append(e.records, records...)
	e.mu.Unlock()
	return nil
}
func (e *captureExporter) Shutdown(context.Context) error { return nil }
func (e *captureExporter) ForceFlush(context.Context) error {
	return nil
}

// TestTraceContextFieldAttachesTraceID verifies that a log emitted through a
// logger carrying TraceContextField ends up with the span's native trace id and
// span id on the exported OTLP record.
func TestTraceContextFieldAttachesTraceID(t *testing.T) {
	cap := &captureExporter{}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(cap)),
	)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	core := otelzap.NewCore("test", otelzap.WithLoggerProvider(provider))

	// The console sink must never see the trace-context field (strip core).
	var console bytes.Buffer
	consoleCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&console),
		zapcore.DebugLevel,
	)
	logger := zap.New(zapcore.NewTee(&stripContextCore{Core: consoleCore}, core))

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	_, span := tp.Tracer("test").Start(context.Background(), "op")

	ctx := trace.ContextWithSpan(context.Background(), span)
	// Only the logger that carries TraceContextField should correlate with the
	// span; a plain logger must stay uncorrelated.
	logger.With(TraceContextField(ctx)).Info("with trace")
	logger.Info("without trace")
	span.End()

	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(cap.records))
	}

	var traced, untraced bool
	for _, r := range cap.records {
		switch r.Body().AsString() {
		case "with trace":
			traced = r.TraceID() == span.SpanContext().TraceID()
		case "without trace":
			untraced = r.TraceID().IsValid()
		}
	}
	if !traced {
		t.Fatal("expected record to carry the span's native trace id")
	}
	if untraced {
		t.Fatal("expected record without context field to have no trace id")
	}
	if strings.Contains(console.String(), "trace.context") {
		t.Fatalf("console output must not contain the trace-context field:\n%s", console.String())
	}
}

// TestIntegrateDisabledNoExport verifies an empty URL disables the pipeline
// without error and the provider is nil.
func TestIntegrateDisabledNoExport(t *testing.T) {
	ctx, provider, err := Integrate(context.Background(), Config{})
	if err != nil {
		t.Fatalf("integrate: %v", err)
	}
	if provider != nil {
		t.Fatal("expected nil provider for empty url")
	}
	log.Of(ctx).Info("noop")
}
