package runtime

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestParseTraceEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    traceEndpoint
		wantErr bool
	}{
		{
			name: "grpc",
			raw:  "grpc://localhost:4317",
			want: traceEndpoint{kind: "grpc", url: "localhost:4317"},
		},
		{
			name: "uppercase scheme is normalized",
			raw:  "GRPC://localhost:4317",
			want: traceEndpoint{kind: "grpc", url: "localhost:4317"},
		},
		{
			name: "jaeger aliases grpc",
			raw:  "jaeger://localhost:4317",
			want: traceEndpoint{kind: "grpc", url: "localhost:4317"},
		},
		{
			name: "grpcs",
			raw:  "grpcs://collector:4317",
			want: traceEndpoint{kind: "grpcs", url: "collector:4317"},
		},
		{
			name: "http",
			raw:  "http://localhost:4318",
			want: traceEndpoint{kind: "http", url: "http://localhost:4318"},
		},
		{ //nolint:gosec // test fixture for basic auth parsing
			name: "http with basic auth",
			raw:  "http://user:pass@collector:4318",
			want: traceEndpoint{kind: "http", url: "http://collector:4318", user: "user", pass: "pass"},
		},
		{ //nolint:gosec // test fixture for basic auth parsing
			name: "https with auth and path",
			raw:  "https://user:pass@collector.example.com:443/v1/traces",
			want: traceEndpoint{kind: "https", url: "https://collector.example.com:443/v1/traces", user: "user", pass: "pass"},
		},
		{ //nolint:gosec // test fixture for basic auth parsing
			name: "grpc with basic auth keeps bare host",
			raw:  "grpc://token:secret@collector:4317",
			want: traceEndpoint{kind: "grpc", url: "collector:4317", user: "token", pass: "secret"},
		},
		{
			name: "http trailing slash is trimmed",
			raw:  "http://collector:4318/",
			want: traceEndpoint{kind: "http", url: "http://collector:4318"},
		},
		{
			name: "custom path trailing slash is trimmed",
			raw:  "http://collector:4318/v1/traces/",
			want: traceEndpoint{kind: "http", url: "http://collector:4318/v1/traces"},
		},
		{
			name:    "scheme-less url is rejected",
			raw:     "localhost:4317",
			wantErr: true,
		},
		{
			name:    "unknown scheme is rejected",
			raw:     "ftp://collector:21",
			wantErr: true,
		},
		{
			name:    "missing address is rejected",
			raw:     "grpc://",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTraceEndpoint(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected %+v, got %+v", tt.want, got)
			}
		})
	}
}

// TestHTTPTraceExporterHeaders verifies the HTTP exporter actually sends the
// basic auth header derived from the URL, any custom headers, and targets the
// OTLP path.
func TestHTTPTraceExporterHeaders(t *testing.T) {
	var mu sync.Mutex
	var gotAuth, gotAPIKey, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("X-Api-Key")
		gotPath = r.URL.Path
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ep, err := parseTraceEndpoint("http://user:pass@" + strings.TrimPrefix(ts.URL, "http://"))
	if err != nil {
		t.Fatalf("parse endpoint: %v", err)
	}
	ep.headers, err = parseTracerHeaders(`{"X-Api-Key":"secret"}`)
	if err != nil {
		t.Fatalf("parse headers: %v", err)
	}

	exp, err := newHTTPTraceExporter(context.Background(), ep)
	if err != nil {
		t.Fatalf("create exporter: %v", err)
	}
	defer func() { _ = exp.Shutdown(context.Background()) }()

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	_, span := tp.Tracer("test").Start(context.Background(), "op")
	if err := exp.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span.(sdktrace.ReadOnlySpan)}); err != nil {
		t.Fatalf("export spans: %v", err)
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	mu.Lock()
	defer mu.Unlock()
	if gotAuth != wantAuth {
		t.Errorf("expected Authorization header %q, got %q", wantAuth, gotAuth)
	}
	if gotAPIKey != "secret" {
		t.Errorf("expected X-Api-Key header %q, got %q", "secret", gotAPIKey)
	}
	if gotPath != "/v1/traces" {
		t.Errorf("expected OTLP path /v1/traces, got %q", gotPath)
	}
}

func TestParseTracerHeaders(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    map[string]string
		wantErr bool
	}{
		{"empty", "", map[string]string{}, false},
		{"whitespace only", "   ", map[string]string{}, false},
		{"json object", `{"api-key":"abc","x-tenant":"42"}`, map[string]string{"api-key": "abc", "x-tenant": "42"}, false},
		{"json with whitespace", ` { "api-key" : "abc" } `, map[string]string{"api-key": "abc"}, false},
		{"empty json object", `{}`, map[string]string{}, false},
		{"json value with comma", `{"a":"b,c"}`, map[string]string{"a": "b,c"}, false},
		{"invalid json", `{"a":`, nil, true},
		{"non-string json value", `{"a":42}`, nil, true},
		{"single pair", "api-key=abc", map[string]string{"api-key": "abc"}, false},
		{"multiple pairs", "api-key=abc,x-tenant=42", map[string]string{"api-key": "abc", "x-tenant": "42"}, false},
		{"padded pairs", " api-key = abc , x-tenant=42 ", map[string]string{"api-key": "abc", "x-tenant": "42"}, false},
		{"trailing comma", "a=b,", map[string]string{"a": "b"}, false},
		{"value with equals", "a=b=c", map[string]string{"a": "b=c"}, false},
		{"empty value", "flag=", map[string]string{"flag": ""}, false},
		{"missing separator", "badpair", nil, true},
		{"empty key", "=value", nil, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTracerHeaders(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Fatalf("expected %v, got %v", tt.want, got)
				}
			}
		})
	}
}

func TestRPCMetadata(t *testing.T) {
	md, err := (rpcMetadata{username: "user", password: "pass", headers: map[string]string{"x-token": "t"}}).GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	if md["x-token"] != "t" {
		t.Errorf("expected x-token metadata %q, got %q", "t", md["x-token"])
	}
	if md["authorization"] != wantAuth {
		t.Errorf("expected authorization metadata %q, got %q", wantAuth, md["authorization"])
	}
}

// TestRPCMetadataAuthorizationOverride verifies an explicitly configured
// authorization header wins over the URL-derived basic auth.
func TestRPCMetadataAuthorizationOverride(t *testing.T) {
	md, err := (rpcMetadata{username: "user", password: "pass", headers: map[string]string{"Authorization": "Bearer custom"}}).GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := ""
	for k, v := range md {
		if strings.EqualFold(k, "authorization") {
			got = v
		}
	}
	if got != "Bearer custom" {
		t.Errorf("expected custom authorization metadata, got %q", got)
	}
}
