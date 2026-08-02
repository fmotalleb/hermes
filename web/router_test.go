package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/fmotalleb/go-tools/log"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func testLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	return zap.New(core), logs
}

func fieldValue(fields []zap.Field, key string) string {
	for _, f := range fields {
		if f.Key != key {
			continue
		}
		switch f.Type {
		case zapcore.StringType:
			return f.String
		case zapcore.Int64Type, zapcore.Int32Type:
			return strconv.FormatInt(f.Integer, 10)
		default:
			return ""
		}
	}
	return ""
}

func TestRequestLoggerGeneratesAndEchoesRequestID(t *testing.T) {
	logger, logs := testLogger()
	r := NewRouter()
	r.RequestLogger()
	r.GET("/zones", func(ctx *Context) (any, error) {
		return map[string]string{"ok": "true"}, nil
	})

	req := httptest.NewRequestWithContext(log.WithLogger(context.Background(), logger), http.MethodGet, "/zones", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	headerID := rec.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Fatal("expected X-Request-ID response header")
	}

	entries := logs.FilterMessage("request").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 request log, got %d", len(entries))
	}
	loggedID := fieldValue(entries[0].Context, "request_id")
	if loggedID == "" {
		t.Fatal("expected request_id field in access log")
	}
	if loggedID != headerID {
		t.Fatalf("logged request id %q does not match response header %q", loggedID, headerID)
	}
}

func TestRequestLoggerRespectsIncomingHeader(t *testing.T) {
	logger, logs := testLogger()
	r := NewRouter()
	r.RequestLogger()
	r.GET("/zones", func(ctx *Context) (any, error) { return nil, nil })

	req := httptest.NewRequestWithContext(log.WithLogger(context.Background(), logger), http.MethodGet, "/zones", nil)
	req.Header.Set("X-Request-ID", "client-supplied-42")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "client-supplied-42" {
		t.Fatalf("expected incoming request id echoed, got %q", got)
	}

	entries := logs.FilterMessage("request").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 request log, got %d", len(entries))
	}
	if got := fieldValue(entries[0].Context, "request_id"); got != "client-supplied-42" {
		t.Fatalf("expected client request id in log, got %q", got)
	}
}

func TestRequestLoggerAddsTraceIDAndEnrichesContext(t *testing.T) {
	logger, logs := testLogger()
	tp := trace.NewTracerProvider()
	ctx, span := tp.Tracer("test").Start(context.Background(), "test-op")
	defer span.End()

	r := NewRouter()
	r.RequestLogger()
	r.GET("/zones", func(ctx *Context) (any, error) {
		// Logging through the context logger must carry the request-scoped ids.
		log.FromContext(ctx).Named("api").Info("handler log")
		return nil, nil
	})

	req := httptest.NewRequestWithContext(log.WithLogger(ctx, logger), http.MethodGet, "/zones", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if len(logs.All()) != 2 {
		t.Fatalf("expected access log + handler log, got %d entries", len(logs.All()))
	}

	wantTraceID := span.SpanContext().TraceID().String()
	for _, entry := range logs.All() {
		if got := fieldValue(entry.Context, "trace_id"); got != wantTraceID {
			t.Fatalf("entry %q: expected trace_id %q, got %q", entry.Message, wantTraceID, got)
		}
		if got := fieldValue(entry.Context, "request_id"); got == "" {
			t.Fatalf("entry %q: expected request_id field", entry.Message)
		}
	}
}

func TestRequestLoggerCoversNotFound(t *testing.T) {
	logger, logs := testLogger()
	r := NewRouter()
	r.RequestLogger()

	req := httptest.NewRequestWithContext(log.WithLogger(context.Background(), logger), http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	entries := logs.FilterMessage("request").All()
	if len(entries) != 1 {
		t.Fatalf("expected 404 request to be logged, got %d entries", len(entries))
	}
	if got := fieldValue(entries[0].Context, "status"); got != "404" {
		t.Fatalf("expected status field 404 in access log, got %q", got)
	}
}
